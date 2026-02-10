package logger

import (
	"testing"
)

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no ANSI codes",
			input: "plain text",
			want:  "plain text",
		},
		{
			name:  "simple color code",
			input: "\x1b[31mRed Text\x1b[0m",
			want:  "Red Text",
		},
		{
			name:  "multiple colors",
			input: "\x1b[31mRed\x1b[0m \x1b[32mGreen\x1b[0m \x1b[34mBlue\x1b[0m",
			want:  "Red Green Blue",
		},
		{
			name:  "complex SGR sequence",
			input: "\x1b[1;31;42mBold Red on Green\x1b[0m",
			want:  "Bold Red on Green",
		},
		{
			name:  "cursor movement",
			input: "Before\x1b[2ACursor Up\x1b[1BAfter",
			want:  "BeforeCursor UpAfter",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only ANSI codes",
			input: "\x1b[31m\x1b[0m\x1b[1m",
			want:  "",
		},
		{
			name:  "real world example",
			input: "\x1b[32m✓\x1b[0m Installation complete",
			want:  "✓ Installation complete",
		},
		{
			name:  "mixed content",
			input: "Normal \x1b[33mWarning:\x1b[0m Something happened",
			want:  "Normal Warning: Something happened",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripANSI(tt.input)
			if got != tt.want {
				t.Errorf("StripANSI() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVisibleLength(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "plain text",
			input: "Hello",
			want:  5,
		},
		{
			name:  "text with color codes",
			input: "\x1b[31mRed\x1b[0m",
			want:  3,
		},
		{
			name:  "multiple colors",
			input: "\x1b[31mR\x1b[32mG\x1b[34mB\x1b[0m",
			want:  3,
		},
		{
			name:  "empty string",
			input: "",
			want:  0,
		},
		{
			name:  "only ANSI codes",
			input: "\x1b[31m\x1b[0m",
			want:  0,
		},
		{
			name:  "unicode emoji",
			input: "Hello 🌙 World",
			want:  13,
		},
		{
			name:  "unicode with ANSI",
			input: "\x1b[33m🌙 Mooncake\x1b[0m",
			want:  10,
		},
		{
			name:  "complex SGR",
			input: "\x1b[1;31;42mText\x1b[0m",
			want:  4,
		},
		{
			name:  "real status line",
			input: "\x1b[32m✓\x1b[0m Step completed successfully",
			want:  29,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VisibleLength(tt.input)
			if got != tt.want {
				t.Errorf("VisibleLength(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncateANSI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
		want     string
	}{
		{
			name:     "no truncation needed",
			input:    "short",
			maxWidth: 10,
			want:     "short",
		},
		{
			name:     "no truncation with ANSI",
			input:    "\x1b[31mshort\x1b[0m",
			maxWidth: 10,
			want:     "\x1b[31mshort\x1b[0m",
		},
		{
			name:     "truncate plain text",
			input:    "this is a very long text",
			maxWidth: 10,
			want:     "this is...",
		},
		{
			name:     "truncate with ANSI codes",
			input:    "\x1b[31mthis is a very long red text\x1b[0m",
			maxWidth: 10,
			want:     "\x1b[31mthis is...",
		},
		{
			name:     "truncate preserves colors",
			input:    "\x1b[32mGreen \x1b[33mYellow \x1b[34mBlue Text\x1b[0m",
			maxWidth: 12,
			want:     "\x1b[32mGreen \x1b[33mYel...",
		},
		{
			name:     "very short width",
			input:    "text",
			maxWidth: 2,
			want:     "te",
		},
		{
			name:     "exact fit",
			input:    "exactly10c",
			maxWidth: 10,
			want:     "exactly10c",
		},
		{
			name:     "exact fit with ANSI",
			input:    "\x1b[31mexactly10c\x1b[0m",
			maxWidth: 10,
			want:     "\x1b[31mexactly10c\x1b[0m",
		},
		{
			name:     "empty string",
			input:    "",
			maxWidth: 10,
			want:     "",
		},
		{
			name:     "unicode truncation",
			input:    "Hello 🌙 World and more text",
			maxWidth: 15,
			want:     "Hello 🌙 Worl...",
		},
		{
			name:     "unicode with ANSI",
			input:    "\x1b[33m🌙 Mooncake is awesome\x1b[0m",
			maxWidth: 12,
			want:     "\x1b[33m🌙 Mooncak...",
		},
		{
			name:     "real world status line",
			input:    "\x1b[32m✓\x1b[0m Step: Install nginx and configure with SSL certificates",
			maxWidth: 30,
			want:     "\x1b[32m✓\x1b[0m Step: Install nginx and c...",
		},
		{
			name:     "preserve all ANSI codes",
			input:    "\x1b[1m\x1b[31m\x1b[42mBold Red on Green Background Long Text\x1b[0m",
			maxWidth: 15,
			want:     "\x1b[1m\x1b[31m\x1b[42mBold Red on ...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateANSI(tt.input, tt.maxWidth)

			// Verify visible length doesn't exceed maxWidth
			visibleLen := VisibleLength(got)
			if visibleLen > tt.maxWidth {
				t.Errorf("TruncateANSI() result has visible length %d, exceeds maxWidth %d", visibleLen, tt.maxWidth)
			}

			// Check expected output
			if got != tt.want {
				t.Errorf("TruncateANSI(%q, %d) = %q, want %q", tt.input, tt.maxWidth, got, tt.want)
				t.Logf("  got visible length: %d", VisibleLength(got))
				t.Logf("  want visible length: %d", VisibleLength(tt.want))
			}
		})
	}
}

func TestExtractVisiblePrefix(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		visibleCount int
		want         string
	}{
		{
			name:         "plain text",
			input:        "Hello World",
			visibleCount: 5,
			want:         "Hello",
		},
		{
			name:         "with ANSI codes",
			input:        "\x1b[31mRed Text Here\x1b[0m",
			visibleCount: 8,
			want:         "\x1b[31mRed Text",
		},
		{
			name:         "preserve all codes",
			input:        "\x1b[1m\x1b[31mBold Red\x1b[0m",
			visibleCount: 4,
			want:         "\x1b[1m\x1b[31mBold",
		},
		{
			name:         "zero count",
			input:        "text",
			visibleCount: 0,
			want:         "",
		},
		{
			name:         "negative count",
			input:        "text",
			visibleCount: -1,
			want:         "",
		},
		{
			name:         "unicode characters",
			input:        "🌙 Moon",
			visibleCount: 3,
			want:         "🌙 M",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractVisiblePrefix(tt.input, tt.visibleCount)
			if got != tt.want {
				t.Errorf("extractVisiblePrefix(%q, %d) = %q, want %q", tt.input, tt.visibleCount, got, tt.want)
			}

			// Verify visible length
			visibleLen := VisibleLength(got)
			if tt.visibleCount > 0 && visibleLen != tt.visibleCount {
				t.Errorf("extractVisiblePrefix() result has %d visible chars, want %d", visibleLen, tt.visibleCount)
			}
		})
	}
}

func TestFindANSIEnd(t *testing.T) {
	tests := []struct {
		name  string
		input string
		pos   int
		want  int
	}{
		{
			name:  "CSI color code",
			input: "\x1b[31mRed",
			pos:   0,
			want:  5,
		},
		{
			name:  "CSI with parameters",
			input: "\x1b[1;31;42m",
			pos:   0,
			want:  10,
		},
		{
			name:  "reset code",
			input: "\x1b[0m",
			pos:   0,
			want:  4,
		},
		{
			name:  "cursor movement",
			input: "\x1b[2A",
			pos:   0,
			want:  4,
		},
		{
			name:  "not at escape",
			input: "text\x1b[31m",
			pos:   0,
			want:  0,
		},
		{
			name:  "at non-escape position",
			input: "abc",
			pos:   1,
			want:  1,
		},
		{
			name:  "simple escape",
			input: "\x1b=",
			pos:   0,
			want:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findANSIEnd(tt.input, tt.pos)
			if got != tt.want {
				t.Errorf("findANSIEnd(%q, %d) = %d, want %d", tt.input, tt.pos, got, tt.want)
			}
		})
	}
}

// Benchmark tests
func BenchmarkStripANSI(b *testing.B) {
	input := "\x1b[31mRed\x1b[0m \x1b[32mGreen\x1b[0m \x1b[34mBlue\x1b[0m"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		StripANSI(input)
	}
}

func BenchmarkVisibleLength(b *testing.B) {
	input := "\x1b[1;31;42mBold Red on Green Background\x1b[0m"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		VisibleLength(input)
	}
}

func BenchmarkTruncateANSI(b *testing.B) {
	input := "\x1b[32m✓\x1b[0m Step: Install nginx and configure with SSL certificates and other things"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		TruncateANSI(input, 30)
	}
}
