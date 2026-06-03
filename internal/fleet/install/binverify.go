package install

// binverify.go guards bootstrap against placing a mooncake binary that
// can't execute on the target. The default --binary is the controller's
// own executable (os.Executable()); when the controller and target differ
// in OS or arch — e.g. a linux/amd64 controller bootstrapping a
// windows/amd64 host — that default is the wrong artefact. Placing it
// anyway produces a confusing post-install failure far from the cause
// (Windows: ERROR_EXE_MACHINE_TYPE_MISMATCH at task-run time). We sniff the
// binary's real format + machine and fail fast with guidance.
//
// Detection is by file format, using only the standard library:
//   ELF    → linux      (mooncake's unix server target)
//   Mach-O → darwin
//   PE     → windows
// Arch comes from the format's machine field (amd64 / arm64 — the two
// mooncake ships).

import (
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"fmt"
	"os"
)

// VerifyBinaryPlatform reads the executable at path far enough to learn
// its OS (by container format) and arch (by machine field), then checks
// both against the target the bootstrap detected. A nil return means the
// binary is safe to place. A descriptive error names the mismatch and how
// to produce the right binary.
//
// On an unrecognised or unreadable file it returns an error rather than
// silently passing — a binary we can't classify is one we shouldn't ship.
func VerifyBinaryPlatform(path, wantOS, wantArch string) error {
	gotOS, gotArch, err := sniffBinaryPlatform(path)
	if err != nil {
		return fmt.Errorf("inspect local binary %q: %w", path, err)
	}
	if gotOS == wantOS && gotArch == wantArch {
		return nil
	}
	return fmt.Errorf(
		"local binary %q is %s/%s but the target is %s/%s; pass --binary "+
			"pointing at a %s/%s mooncake build (e.g. GOOS=%s GOARCH=%s go build -o mooncake%s ./cmd)",
		path, gotOS, gotArch, wantOS, wantArch,
		wantOS, wantArch, wantOS, wantArch, exeSuffix(wantOS))
}

// sniffBinaryPlatform classifies the file at path by its executable
// container format. Each opener validates the magic, so we try them in
// turn and use whichever accepts the file.
func sniffBinaryPlatform(path string) (goos, goarch string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = f.Close() }()

	if ef, e := elf.NewFile(f); e == nil {
		switch ef.Machine {
		case elf.EM_X86_64:
			return "linux", "amd64", nil
		case elf.EM_AARCH64:
			return "linux", "arm64", nil
		default:
			return "linux", fmt.Sprintf("elf-machine-%d", ef.Machine), nil
		}
	}
	if _, e := f.Seek(0, 0); e != nil {
		return "", "", e
	}
	if mf, e := macho.NewFile(f); e == nil {
		switch mf.Cpu {
		case macho.CpuAmd64:
			return "darwin", "amd64", nil
		case macho.CpuArm64:
			return "darwin", "arm64", nil
		default:
			return "darwin", fmt.Sprintf("macho-cpu-%d", mf.Cpu), nil
		}
	}
	if _, e := f.Seek(0, 0); e != nil {
		return "", "", e
	}
	if pf, e := pe.NewFile(f); e == nil {
		switch pf.Machine {
		case pe.IMAGE_FILE_MACHINE_AMD64:
			return "windows", "amd64", nil
		case pe.IMAGE_FILE_MACHINE_ARM64:
			return "windows", "arm64", nil
		default:
			return "windows", fmt.Sprintf("pe-machine-%d", pf.Machine), nil
		}
	}
	return "", "", fmt.Errorf("unrecognised executable format (not ELF, Mach-O, or PE)")
}

// exeSuffix returns the conventional executable extension for goos, used
// only to make the suggested `go build -o` command in the error copy-paste
// correct.
func exeSuffix(goos string) string {
	if goos == "windows" {
		return ".exe"
	}
	return ""
}
