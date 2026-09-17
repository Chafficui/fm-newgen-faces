// Package assign turns parsed players + a face pack + a config.xml into a
// Plan (dry run, nothing written) and then applies it.
package assign

import (
	"fmt"
	"math/rand"
	"time"

	"fmnewgenfaces/internal/core/ethnic"
	"fmnewgenfaces/internal/core/facepack"
	"fmnewgenfaces/internal/core/fmconfig"
	"fmnewgenfaces/internal/core/rtf"
)

// Options mirror the user settings.
type Options struct {
	// Preserve keeps existing newgen mappings; only players without one get a face.
	Preserve bool
	// AllowDuplicates lets one image be used for several players. When false,
	// images already mapped in the config are excluded and each draw consumes.
	AllowDuplicates bool
	// Seed makes runs reproducible (0 = random).
	Seed int64
}

// Demand summarises one ethnic group in a plan.
type Demand struct {
	Ethnic    ethnic.Ethnic
	Needed    int // players that will receive a new face
	Available int // images usable for them (after exclusion when !AllowDuplicates)
	Preserved int // players kept as-is
}

// Shortfall returns max(0, Needed-Available) when duplicates are not allowed, else 0.
func (d Demand) Shortfall(allowDuplicates bool) int {
	if allowDuplicates || d.Available >= d.Needed {
		return 0
	}
	return d.Needed - d.Available
}

// Plan is the dry-run result.
type Plan struct {
	Options   Options
	New       []rtf.Player // will receive a face
	Preserved []rtf.Player // already mapped and Preserve=true
	Unmapped  []rtf.Player // no ethnic (needs override) – never assigned
	PerEthnic map[ethnic.Ethnic]*Demand
	// Excluded is the number of pack images removed from the pool because
	// they are already mapped (only when !AllowDuplicates).
	Excluded int
}

// TotalShortfall sums Demand.Shortfall over all groups.
func (p *Plan) TotalShortfall() int {
	total := 0
	for _, d := range p.PerEthnic {
		total += d.Shortfall(p.Options.AllowDuplicates)
	}
	return total
}

// excludedRefs returns the ImageRefs from cfg.AssignedImages() that
// facepack.ParseRef recognises.
func excludedRefs(cfg *fmconfig.Config) []facepack.ImageRef {
	var out []facepack.ImageRef
	for _, from := range cfg.AssignedImages() {
		if ref, ok := facepack.ParseRef(from); ok {
			out = append(out, ref)
		}
	}
	return out
}

// Build computes a Plan without touching cfg or the filesystem.
func Build(res *rtf.Result, cfg *fmconfig.Config, pack *facepack.Pack, opt Options) *Plan {
	plan := &Plan{
		Options:   opt,
		Unmapped:  append([]rtf.Player(nil), res.UnmappedPlayers...),
		PerEthnic: make(map[ethnic.Ethnic]*Demand),
	}

	for _, p := range res.Players {
		if opt.Preserve && cfg.Has(p.ID) {
			plan.Preserved = append(plan.Preserved, p)
		} else {
			plan.New = append(plan.New, p)
		}
	}

	excludedByEthnic := make(map[ethnic.Ethnic]int)
	if !opt.AllowDuplicates {
		for _, ref := range excludedRefs(cfg) {
			excludedByEthnic[ref.Ethnic]++
			plan.Excluded++
		}
	}

	needed := make(map[ethnic.Ethnic]int)
	preserved := make(map[ethnic.Ethnic]int)
	for _, p := range plan.New {
		needed[p.Ethnic]++
	}
	for _, p := range plan.Preserved {
		preserved[p.Ethnic]++
	}

	for _, e := range ethnic.All {
		n := needed[e]
		pr := preserved[e]
		if n == 0 && pr == 0 {
			continue
		}
		available := pack.Count(e) - excludedByEthnic[e]
		if available < 0 {
			available = 0
		}
		plan.PerEthnic[e] = &Demand{
			Ethnic:    e,
			Needed:    n,
			Available: available,
			Preserved: pr,
		}
	}

	return plan
}

// Assignment is one player→image decision.
type Assignment struct {
	Player rtf.Player
	Image  string // base name
	From   string // value written to config.xml
}

// Skipped is a player that could not be given a face.
type Skipped struct {
	Player rtf.Player
	Reason string
}

// Result is what Apply did (cfg is mutated but NOT saved).
type Result struct {
	Assigned  []Assignment
	Preserved int
	Skipped   []Skipped
	Unmapped  int
}

// Progress is called during Apply with done/total counts; may be nil.
type Progress func(done, total int)

func newRNG(seed int64) *rand.Rand {
	if seed != 0 {
		return rand.New(rand.NewSource(seed))
	}
	return rand.New(rand.NewSource(time.Now().UnixNano()))
}

// Apply mutates cfg according to plan. It never writes to disk.
func Apply(plan *Plan, cfg *fmconfig.Config, pack *facepack.Pack, progress Progress) (*Result, error) {
	pool := facepack.NewPool(pack, newRNG(plan.Options.Seed))

	if !plan.Options.AllowDuplicates {
		pool.Exclude(excludedRefs(cfg))
	}

	result := &Result{
		Preserved: len(plan.Preserved),
		Unmapped:  len(plan.Unmapped),
	}

	total := len(plan.New)
	consume := !plan.Options.AllowDuplicates

	for i, p := range plan.New {
		img, err := pool.Draw(p.Ethnic, consume)
		if err != nil {
			result.Skipped = append(result.Skipped, Skipped{
				Player: p,
				Reason: fmt.Sprintf("no images left for %s", p.Ethnic),
			})
		} else {
			from, ferr := fmconfig.ImagePath(cfg.Path, pack.Root, p.Ethnic, img)
			if ferr != nil {
				return nil, ferr
			}
			cfg.Set(p.ID, from)
			result.Assigned = append(result.Assigned, Assignment{
				Player: p,
				Image:  img,
				From:   from,
			})
		}

		done := i + 1
		if progress != nil && (done%100 == 0 || done == total) {
			progress(done, total)
		}
	}

	if progress != nil && total == 0 {
		progress(0, 0)
	}

	return result, nil
}

// Reroll draws a different image for one player and updates cfg. With
// AllowDuplicates=false the previous image goes back into circulation and the
// new one must not be mapped to anyone else.
func Reroll(cfg *fmconfig.Config, pack *facepack.Pack, p rtf.Player, opt Options) (Assignment, error) {
	current, hasCurrent := cfg.Get(p.ID)
	var currentRef facepack.ImageRef
	haveCurrentRef := false
	if hasCurrent {
		if ref, ok := facepack.ParseRef(current); ok {
			currentRef = ref
			haveCurrentRef = true
		}
	}

	pool := facepack.NewPool(pack, newRNG(opt.Seed))

	if !opt.AllowDuplicates {
		var used []facepack.ImageRef
		for uid, from := range cfg.Newgens() {
			if uid == p.ID {
				continue // the player's own current image goes back into circulation
			}
			if ref, ok := facepack.ParseRef(from); ok {
				used = append(used, ref)
			}
		}
		pool.Exclude(used)
	}

	// Only force a different image than the current one when there is at
	// least one other option; otherwise the sole remaining image (the
	// current one) is drawn again.
	if haveCurrentRef && pool.Available(p.Ethnic) > 1 {
		pool.Exclude([]facepack.ImageRef{currentRef})
	}

	img, err := pool.Draw(p.Ethnic, false)
	if err != nil {
		return Assignment{}, err
	}

	from, err := fmconfig.ImagePath(cfg.Path, pack.Root, p.Ethnic, img)
	if err != nil {
		return Assignment{}, err
	}

	cfg.Set(p.ID, from)

	return Assignment{Player: p, Image: img, From: from}, nil
}
