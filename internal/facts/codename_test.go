package facts

import "testing"

func TestParseOSRelease(t *testing.T) {
	cases := []struct {
		name         string
		input        string
		wantID       string
		wantVersion  string
		wantCodename string
	}{
		{
			name: "ubuntu_jammy",
			input: `NAME="Ubuntu"
VERSION="22.04.4 LTS (Jammy Jellyfish)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 22.04.4 LTS"
VERSION_ID="22.04"
VERSION_CODENAME=jammy
UBUNTU_CODENAME=jammy
`,
			wantID:       "ubuntu",
			wantVersion:  "22.04",
			wantCodename: "jammy",
		},
		{
			name: "debian_bookworm_quoted",
			input: `PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
ID=debian
VERSION_ID="12"
VERSION_CODENAME="bookworm"
`,
			wantID:       "debian",
			wantVersion:  "12",
			wantCodename: "bookworm",
		},
		{
			name: "lsb_release_fallback",
			input: `DISTRIB_ID=Ubuntu
DISTRIB_RELEASE=20.04
DISTRIB_CODENAME=focal
DISTRIB_DESCRIPTION="Ubuntu 20.04.6 LTS"
`,
			wantID:       "",
			wantVersion:  "",
			wantCodename: "focal",
		},
		{
			name: "version_codename_wins_over_distrib_codename",
			input: `ID=ubuntu
VERSION_ID="22.04"
DISTRIB_CODENAME=ignored
VERSION_CODENAME=jammy
`,
			wantID:       "ubuntu",
			wantVersion:  "22.04",
			wantCodename: "jammy",
		},
		{
			name: "no_codename",
			input: `ID=alpine
VERSION_ID=3.18.4
`,
			wantID:       "alpine",
			wantVersion:  "3.18.4",
			wantCodename: "",
		},
		{
			name:         "empty",
			input:        "",
			wantID:       "",
			wantVersion:  "",
			wantCodename: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, ver, codename := parseOSRelease([]byte(tc.input))
			if id != tc.wantID {
				t.Errorf("id = %q, want %q", id, tc.wantID)
			}
			if ver != tc.wantVersion {
				t.Errorf("version = %q, want %q", ver, tc.wantVersion)
			}
			if codename != tc.wantCodename {
				t.Errorf("codename = %q, want %q", codename, tc.wantCodename)
			}
		})
	}
}
