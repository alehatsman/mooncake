package windows_registry

import (
	"encoding/json"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

func TestHandler_Validate(t *testing.T) {
	h := &Handler{}
	cases := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{"nil config", &config.Step{}, true},
		{"missing path", &config.Step{WindowsRegistry: &config.WindowsRegistry{}}, true},
		{"bad state", &config.Step{WindowsRegistry: &config.WindowsRegistry{Path: `HKLM:\SOFTWARE\Foo`, State: "maybe"}}, true},
		{"value without value field", &config.Step{WindowsRegistry: &config.WindowsRegistry{
			Path: `HKLM:\SOFTWARE\Foo`, Name: "Bar",
		}}, true},
		{"bad type", &config.Step{WindowsRegistry: &config.WindowsRegistry{
			Path: `HKLM:\SOFTWARE\Foo`, Name: "Bar", Value: "1", Type: "unknown",
		}}, true},
		{"happy key present", &config.Step{WindowsRegistry: &config.WindowsRegistry{
			Path: `HKLM:\SOFTWARE\Foo`,
		}}, false},
		{"happy key absent", &config.Step{WindowsRegistry: &config.WindowsRegistry{
			Path: `HKLM:\SOFTWARE\Foo`, State: "absent",
		}}, false},
		{"happy string value", &config.Step{WindowsRegistry: &config.WindowsRegistry{
			Path: `HKLM:\SOFTWARE\Foo`, Name: "Bar", Value: "hello",
		}}, false},
		{"happy dword value", &config.Step{WindowsRegistry: &config.WindowsRegistry{
			Path: `HKLM:\SOFTWARE\Foo`, Name: "Count", Value: "42", Type: "dword",
		}}, false},
		{"absent value no value field ok", &config.Step{WindowsRegistry: &config.WindowsRegistry{
			Path: `HKLM:\SOFTWARE\Foo`, Name: "Bar", State: "absent",
		}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := h.Validate(tc.step)
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestNormalizeType(t *testing.T) {
	cases := map[string]string{
		"":              "String",
		"string":        "String",
		"dword":         "DWord",
		"DWORD":         "DWord",
		"qword":         "QWord",
		"expand_string": "ExpandString",
		"multi_string":  "MultiString",
		"binary":        "Binary",
	}
	for in, want := range cases {
		if got := normalizeType(in); got != want {
			t.Errorf("normalizeType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeValueForCompare(t *testing.T) {
	cases := []struct {
		value, typ, want string
	}{
		{"hello", "string", "hello"},
		{"0A-1B-2C", "binary", "0a1b2c"},
		{"0a1b2c", "binary", "0a1b2c"},
		{"a\r\nb", "multi_string", "a\nb"},
		{"a\nb", "multi_string", "a\nb"},
	}
	for _, tc := range cases {
		if got := normalizeValueForCompare(tc.value, tc.typ); got != tc.want {
			t.Errorf("normalizeValueForCompare(%q, %q) = %q, want %q", tc.value, tc.typ, got, tc.want)
		}
	}
}

func TestQueryKeyExists_Mock(t *testing.T) {
	orig := runPS
	defer func() { runPS = orig }()

	runPS = func(script string) (string, error) {
		return "yes\r\n", nil
	}
	ok, err := queryKeyExists(`HKLM:\SOFTWARE\Foo`)
	if err != nil || !ok {
		t.Errorf("expected true, got ok=%v err=%v", ok, err)
	}

	runPS = func(script string) (string, error) {
		return "no\r\n", nil
	}
	ok, err = queryKeyExists(`HKLM:\SOFTWARE\Foo`)
	if err != nil || ok {
		t.Errorf("expected false, got ok=%v err=%v", ok, err)
	}
}

func TestQueryValue_Mock(t *testing.T) {
	orig := runPS
	defer func() { runPS = orig }()

	t.Run("key missing", func(t *testing.T) {
		runPS = func(_ string) (string, error) {
			b, _ := json.Marshal(map[string]interface{}{"KeyExists": false, "ValueExists": false})
			return string(b), nil
		}
		obs, err := queryValue(`HKLM:\SOFTWARE\Foo`, "Bar")
		if err != nil || obs.KeyExists || obs.ValueExists {
			t.Errorf("unexpected: %+v %v", obs, err)
		}
	})

	t.Run("value present", func(t *testing.T) {
		runPS = func(_ string) (string, error) {
			b, _ := json.Marshal(map[string]interface{}{
				"KeyExists": true, "ValueExists": true, "Value": "hello", "Kind": "String",
			})
			return string(b), nil
		}
		obs, err := queryValue(`HKLM:\SOFTWARE\Foo`, "Bar")
		if err != nil || !obs.ValueExists || obs.Value != "hello" || obs.Kind != "String" {
			t.Errorf("unexpected: %+v %v", obs, err)
		}
	})
}

func TestRenderPSValue(t *testing.T) {
	cases := []struct {
		value, typ, want string
	}{
		{"hello", "string", "'hello'"},
		{"hello", "", "'hello'"},
		{"42", "dword", "[int]'42'"},
		{"100", "qword", "[long]'100'"},
		{"foo\nbar", "multi_string", "@('foo','bar')"},
	}
	for _, tc := range cases {
		if got := renderPSValue(tc.value, tc.typ); got != tc.want {
			t.Errorf("renderPSValue(%q, %q) = %q, want %q", tc.value, tc.typ, got, tc.want)
		}
	}
}

func TestPsQuote(t *testing.T) {
	cases := map[string]string{
		"hello":       "'hello'",
		"it's a test": "'it''s a test'",
		"":            "''",
	}
	for in, want := range cases {
		if got := psQuote(in); got != want {
			t.Errorf("psQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
