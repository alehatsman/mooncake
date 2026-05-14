package recommend

// entry pairs a preset name with the profile under which it should be
// suggested. base=true means "always recommend, regardless of profile".
//
// Curated, not exhaustive. Order matters: base entries are emitted
// first, then profile-specific entries in declaration order. The list
// is intentionally short — too many recommendations and the user is
// back where they started.
type entry struct {
	name    string
	base    bool
	profile Profile
}

var catalogue = []entry{
	// Base — useful on every profile.
	{name: "git", base: true},
	{name: "zsh", base: true},
	{name: "tmux", base: true},
	{name: "neovim", base: true},
	{name: "fzf", base: true},
	{name: "ripgrep", base: true},

	// Linux + apt (Debian / Ubuntu).
	{name: "docker", profile: Profile{OS: "linux", PackageManager: "apt"}},
	{name: "ufw", profile: Profile{OS: "linux", PackageManager: "apt"}},
	{name: "build-essential", profile: Profile{OS: "linux", PackageManager: "apt"}},

	// Linux + pacman (Arch / Manjaro).
	{name: "docker", profile: Profile{OS: "linux", PackageManager: "pacman"}},
	{name: "base-devel", profile: Profile{OS: "linux", PackageManager: "pacman"}},

	// Linux + dnf (Fedora / RHEL).
	{name: "docker", profile: Profile{OS: "linux", PackageManager: "dnf"}},
	{name: "development-tools", profile: Profile{OS: "linux", PackageManager: "dnf"}},

	// macOS (Homebrew).
	{name: "homebrew", profile: Profile{OS: "darwin"}},
	{name: "iterm2", profile: Profile{OS: "darwin"}},
	{name: "rectangle", profile: Profile{OS: "darwin", PackageManager: "brew"}},

	// Generic Linux (no specific package manager match).
	{name: "sshd-hardening", profile: Profile{OS: "linux"}},
}
