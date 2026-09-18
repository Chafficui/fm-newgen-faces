// Package brand centralises every user-visible product name so a rename is a
// one-file change. Version is injected at build time via
// -ldflags "-X fmnewgenfaces/internal/brand.Version=1.2.3".
package brand

const (
	// AppName is the full product name shown in window titles and docs.
	AppName = "FM NewGen Faces"
	// ShortName is used where space is tight (tabs, notifications).
	ShortName = "NewGen Faces"
	// BinaryName is the executable / archive base name.
	BinaryName = "fm-newgen-faces"
	// AppID is the Fyne application ID (reverse-DNS, used for preferences).
	AppID = "io.github.chafficui.fmnewgenfaces"
	// ConfigDirName is the folder under the OS user-config dir.
	ConfigDirName = "fm-newgen-faces"
	// LegacyConfigDirName is the pre-2.0 folder ("jaqen") that is migrated on first start.
	LegacyConfigDirName = "jaqen"

	RepoOwner   = "chafficui"
	RepoName    = "fm-newgen-faces"
	RepoURL     = "https://github.com/" + RepoOwner + "/" + RepoName
	ReleasesURL = RepoURL + "/releases"
	IssuesURL   = RepoURL + "/issues/new"
	TutorialURL = "https://youtu.be/aHnrpfH--ic"

	// LogFileName is the log written into the config dir.
	LogFileName = "fm-newgen-faces.log"
)

// Version is the semantic version of this build ("dev" when built locally).
var Version = "dev"
