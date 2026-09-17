// Package rtf parses the "SCRIPT FACES player search" export that Football
// Manager writes when the user prints the newgen search to a text file.
//
// Each player row looks like:
//
//	| 2000133469| GER       | RSA       | Tebogo Maluleke            | 1         | 16        | 3         |
//
// Columns after splitting on "|": [0]="" [1]=UID [2]=Nat [3]=2nd Nat [4]=Name
// [5],[6]=unused [7]=ethnicity value (0..10). Separator rows contain only
// dashes. Files are CRLF and may contain RTF control words before the table.
package rtf

import (
	"bufio"
	"errors"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"fmnewgenfaces/internal/core/ethnic"
)

// uidPattern finds the 7+ digit UID inside an already-isolated column. It is
// deliberately applied to column 1 alone (never to the whole raw line) so a
// player name containing a long run of digits is never mistaken for a UID.
var uidPattern = regexp.MustCompile(`\d{7,}`)

// Player is one newgen row. Ethnic is empty when the nation could not be
// resolved (see Result.Unmapped).
type Player struct {
	ID          string
	Name        string
	Nation      string
	Nation2     string
	EthnicValue int
	Ethnic      ethnic.Ethnic
}

// Issue describes a row that could not be parsed.
type Issue struct {
	Line   int
	Text   string
	Reason string
}

// Result is the outcome of Parse. Parse never fails because of a single bad
// row: bad rows go to Malformed, players with unknown nations go to Unmapped
// AND UnmappedPlayers, and everything else goes to Players.
type Result struct {
	Path string
	// Players have a resolved Ethnic and are unique by ID (first occurrence wins).
	Players []Player
	// UnmappedPlayers have Ethnic == "" because Nation is unknown to the resolver.
	UnmappedPlayers []Player
	// Unmapped counts how many players are blocked by each unknown nation code.
	Unmapped  map[string]int
	Malformed []Issue
	// Duplicates is the number of rows dropped because their ID was already seen.
	Duplicates int
}

// Total returns len(Players)+len(UnmappedPlayers).
func (r *Result) Total() int { return len(r.Players) + len(r.UnmappedPlayers) }

// UnmappedCodes returns the unknown nation codes sorted by count desc, then name.
func (r *Result) UnmappedCodes() []string {
	codes := make([]string, 0, len(r.Unmapped))
	for c := range r.Unmapped {
		codes = append(codes, c)
	}
	sort.Slice(codes, func(i, j int) bool {
		ci, cj := codes[i], codes[j]
		if r.Unmapped[ci] != r.Unmapped[cj] {
			return r.Unmapped[ci] > r.Unmapped[cj]
		}
		return ci < cj
	})
	return codes
}

// ErrNoPlayers is returned when the file contains no player rows at all
// (wrong file, or a print of a different view).
var ErrNoPlayers = errNoPlayers{}

type errNoPlayers struct{}

func (errNoPlayers) Error() string { return "no player rows found in file" }

// Parse reads the export at path and resolves ethnic groups with res.
// It returns an error only for I/O problems or ErrNoPlayers.
func Parse(path string, res *ethnic.Resolver) (*Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := &Result{
		Path:     path,
		Unmapped: make(map[string]int),
	}

	seen := make(map[string]bool)
	rowsFound := 0

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		rawLine := strings.TrimRight(scanner.Text(), "\r")

		// Rows that aren't even table rows (RTF control words before the
		// table, blank lines, ...) are skipped silently.
		cols := strings.Split(rawLine, "|")
		if len(cols) < 2 {
			continue
		}

		// A row is only a player-row candidate when its UID column (column
		// 1) holds a 7+ digit number. This also skips the header row (col 1
		// is "UID") and dashed separator rows without flagging them as
		// malformed.
		uid := uidPattern.FindString(strings.TrimSpace(cols[1]))
		if uid == "" {
			continue
		}
		rowsFound++

		trimmed := make([]string, len(cols))
		for i, c := range cols {
			trimmed[i] = strings.TrimSpace(c)
		}

		if len(trimmed) < 8 {
			result.Malformed = append(result.Malformed, Issue{
				Line:   lineNo,
				Text:   rawLine,
				Reason: "fewer than 8 columns",
			})
			continue
		}

		ethnicValue, convErr := strconv.Atoi(trimmed[7])
		if convErr != nil {
			result.Malformed = append(result.Malformed, Issue{
				Line:   lineNo,
				Text:   rawLine,
				Reason: "non-integer ethnic column: " + trimmed[7],
			})
			continue
		}

		if seen[uid] {
			result.Duplicates++
			continue
		}
		seen[uid] = true

		nation1 := trimmed[2]
		nation2 := trimmed[3]
		name := trimmed[4]

		player := Player{
			ID:          uid,
			Name:        name,
			Nation:      nation1,
			Nation2:     nation2,
			EthnicValue: ethnicValue,
		}

		eth, resolveErr := res.Resolve(nation1, nation2, ethnicValue)
		if resolveErr != nil {
			var unknownNation *ethnic.UnknownNationError
			if errors.As(resolveErr, &unknownNation) {
				result.UnmappedPlayers = append(result.UnmappedPlayers, player)
				result.Unmapped[unknownNation.Code]++
				continue
			}
			result.Malformed = append(result.Malformed, Issue{
				Line:   lineNo,
				Text:   rawLine,
				Reason: resolveErr.Error(),
			})
			continue
		}

		player.Ethnic = eth
		result.Players = append(result.Players, player)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if rowsFound == 0 {
		return nil, ErrNoPlayers
	}

	return result, nil
}

// Resolve re-runs ethnic resolution on an existing Result with a new resolver
// (used after the user adds overrides): players move from UnmappedPlayers to
// Players when they now resolve.
func (r *Result) Resolve(res *ethnic.Resolver) {
	stillUnmapped := make([]Player, 0, len(r.UnmappedPlayers))
	newUnmapped := make(map[string]int)

	for _, p := range r.UnmappedPlayers {
		eth, err := res.Resolve(p.Nation, p.Nation2, p.EthnicValue)
		if err != nil {
			stillUnmapped = append(stillUnmapped, p)
			var unknownNation *ethnic.UnknownNationError
			if errors.As(err, &unknownNation) {
				newUnmapped[unknownNation.Code]++
			}
			continue
		}
		p.Ethnic = eth
		r.Players = append(r.Players, p)
	}

	r.UnmappedPlayers = stillUnmapped
	r.Unmapped = newUnmapped
}
