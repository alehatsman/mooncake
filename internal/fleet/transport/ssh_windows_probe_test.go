package transport

import (
	"strings"
	"testing"
)

func TestParseWindowsProbe_HappyPath(t *testing.T) {
	cases := []struct {
		name string
		in   string
		os   string
		arch string
	}{
		{"amd64", "Windows\r\nAMD64\r\n", "windows", "amd64"},
		{"x86_64", "Windows\nx86_64\n", "windows", "amd64"},
		{"arm64", "Windows\nARM64\n", "windows", "arm64"},
		{"aarch64", "Windows\naarch64\n", "windows", "arm64"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			os, arch, err := parseWindowsProbe(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if os != tc.os || arch != tc.arch {
				t.Errorf("got (%q, %q), want (%q, %q)", os, arch, tc.os, tc.arch)
			}
		})
	}
}

func TestParseWindowsProbe_RejectsBadInput(t *testing.T) {
	cases := []string{
		"",
		"only-one-line",
		"Linux\nx86_64",  // wrong OS marker
		"Windows\nIA64",  // unsupported arch
		"Windows\nx86\n", // 32-bit not in scope
		"WSL\nAMD64",     // OS marker is "WSL" not "Windows"
	}
	for _, in := range cases {
		t.Run(strings.ReplaceAll(in, "\n", "|"), func(t *testing.T) {
			if _, _, err := parseWindowsProbe(in); err == nil {
				t.Fatalf("expected error for %q", in)
			}
		})
	}
}
