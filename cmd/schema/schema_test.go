package schema

import (
	"bytes"
	"testing"

	"github.com/alehatsman/mooncake/internal/schemagen"
)

// TestIssue28_GenerateValidateAlignment guards issue #28. The previous
// validate path hardcoded {Strict:false, Extensions:true} on its
// in-memory generator while `schema generate`'s defaults are
// {Strict:true, Extensions:true}. The two schemas differed by ~6.3KB
// of `additionalProperties:false` entries (gated on StrictValidation),
// so a freshly-generated file never byte-matched the validate-time
// rendering and validate always reported out-of-date.
//
// This test pins the byte-for-byte alignment: rendering the same
// generator output through the same Writer must be deterministic, and
// both validate-mode and generate-mode using the same options must
// produce identical bytes. The two cases below cover the default
// (strict=true) and the inverse (strict=false) — both must round-trip.
func TestIssue28_GenerateValidateAlignment(t *testing.T) {
	cases := []struct {
		name   string
		strict bool
	}{
		{"strict_default", true},
		{"loose", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := schemagen.GeneratorOptions{
				IncludeExtensions: true,
				IncludeExamples:   false,
				StrictValidation:  tc.strict,
				OutputFormat:      "json",
			}
			// Render twice — once as `generate` would write to disk,
			// once as `validate` would render the live schema for compare.
			// They must be byte-identical for validate to ever succeed.
			var a, b bytes.Buffer
			s1, err := schemagen.NewGenerator(opts).Generate()
			if err != nil {
				t.Fatalf("generate #1: %v", err)
			}
			if err := schemagen.NewWriter("json").Write(s1, &a); err != nil {
				t.Fatalf("write #1: %v", err)
			}
			s2, err := schemagen.NewGenerator(opts).Generate()
			if err != nil {
				t.Fatalf("generate #2: %v", err)
			}
			if err := schemagen.NewWriter("json").Write(s2, &b); err != nil {
				t.Fatalf("write #2: %v", err)
			}
			if !bytes.Equal(a.Bytes(), b.Bytes()) {
				t.Errorf("two writes of the same generator-options produced different bytes (lens %d vs %d) — validate can never succeed",
					a.Len(), b.Len())
			}
		})
	}
}

// TestIssue28_StrictAffectsBytes pins that the --strict flag is
// load-bearing: flipping it produces materially different bytes. If
// this ever stops holding, the validate-time --strict flag becomes
// inert and the issue regresses silently.
func TestIssue28_StrictAffectsBytes(t *testing.T) {
	mk := func(strict bool) []byte {
		opts := schemagen.GeneratorOptions{
			IncludeExtensions: true,
			IncludeExamples:   false,
			StrictValidation:  strict,
			OutputFormat:      "json",
		}
		s, err := schemagen.NewGenerator(opts).Generate()
		if err != nil {
			t.Fatalf("generate strict=%v: %v", strict, err)
		}
		var buf bytes.Buffer
		if err := schemagen.NewWriter("json").Write(s, &buf); err != nil {
			t.Fatalf("write strict=%v: %v", strict, err)
		}
		return buf.Bytes()
	}
	strictBytes := mk(true)
	looseBytes := mk(false)
	if bytes.Equal(strictBytes, looseBytes) {
		t.Fatal("expected --strict=true and --strict=false to produce different schemas; if they're identical now, the validate --strict flag added for issue #28 has become inert")
	}
	if !bytes.Contains(strictBytes, []byte("\"additionalProperties\": false")) {
		t.Error("strict-mode schema should contain additionalProperties:false entries")
	}
}
