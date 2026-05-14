package executor

// secret_resolve.go implements the apply-time half of spec-23 §3
// (`!secret <ref>`). The YAML pre-pass in internal/config/secret_tag.go
// rewrites tagged scalars to sentinel-marker strings that flow through
// plan compilation untouched. Just before each handler runs, we walk
// the step's action struct fields, replace any marker with the resolved
// value from security.DefaultRegistry, and add the resolved value to
// the run's Redactor denylist so it can't leak into events / runlog /
// step.stdout.
//
// Plan mode (ec.Mode() == actions.ModePlan) is intentionally a no-op —
// the markers stay in-memory and get rewritten as `"!secret <ref>"`
// when the planner serializes JSON for `mooncake plan --format json`.

import (
	"fmt"
	"reflect"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/security"
)

// resolveStepSecrets walks step's action fields, resolves any sentinel-
// marker strings to actual secret values, and adds the resolved values
// to ec.Redactor. Mutates step in place — the caller has already cloned
// the step out of the plan if it cares about isolation.
//
// Returns the first resolution error encountered; the underlying
// provider error is wrapped with the offending field's path for
// debuggability. The error redacts the secret path beyond the provider
// prefix (handled in security.Registry.Resolve).
func resolveStepSecrets(step *config.Step, ec *ExecutionContext) error {
	if ec.Mode() == actions.ModePlan {
		return nil
	}
	if step == nil {
		return nil
	}
	var redactor *security.Redactor
	if ec.Svc != nil {
		redactor = ec.Svc.Redactor
	}
	return walkAndResolveSecrets(reflect.ValueOf(step).Elem(), redactor)
}

// walkAndResolveSecrets mirrors the shape of planner.walkAndRender —
// same set of supported field kinds (string, *string, *struct, []string,
// map[string]string) — but rewrites only the strings carrying the
// sentinel marker. Other strings are left alone.
//
// Lives in the executor package rather than plan/ because resolution
// is an apply-time concern and depends on the per-run Redactor.
func walkAndResolveSecrets(rv reflect.Value, redactor *security.Redactor) error {
	if !rv.IsValid() {
		return nil
	}
	if rv.Kind() != reflect.Struct {
		return nil
	}
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		fv := rv.Field(i)
		sf := rt.Field(i)

		switch fv.Kind() {
		case reflect.String:
			if err := resolveStringInPlace(fv, sf.Name, redactor); err != nil {
				return err
			}

		case reflect.Pointer:
			if fv.IsNil() {
				continue
			}
			switch fv.Type().Elem().Kind() {
			case reflect.String:
				if err := resolveStringInPlace(fv.Elem(), sf.Name, redactor); err != nil {
					return err
				}
			case reflect.Struct:
				if err := walkAndResolveSecrets(fv.Elem(), redactor); err != nil {
					return err
				}
			}

		case reflect.Slice:
			if fv.Type().Elem().Kind() != reflect.String {
				continue
			}
			for j := 0; j < fv.Len(); j++ {
				if err := resolveStringInPlace(fv.Index(j), fmt.Sprintf("%s[%d]", sf.Name, j), redactor); err != nil {
					return err
				}
			}

		case reflect.Map:
			if fv.Type().Key().Kind() != reflect.String || fv.Type().Elem().Kind() != reflect.String {
				continue
			}
			for _, k := range fv.MapKeys() {
				cur := fv.MapIndex(k).String()
				if !security.IsMarker(cur) {
					continue
				}
				val, _, err := security.ResolveMarker(cur)
				if err != nil {
					return fmt.Errorf("%s[%s]: %w", sf.Name, k.String(), err)
				}
				if redactor != nil {
					redactor.AddSensitive(val)
				}
				fv.SetMapIndex(k, reflect.ValueOf(val))
			}
		}
	}
	return nil
}

// resolveStringInPlace runs the marker check + provider resolve + redactor
// add on a single string-valued reflect.Value, mutating it in place.
// fieldName is used only for error context.
func resolveStringInPlace(fv reflect.Value, fieldName string, redactor *security.Redactor) error {
	cur := fv.String()
	if !security.IsMarker(cur) {
		return nil
	}
	val, _, err := security.ResolveMarker(cur)
	if err != nil {
		return fmt.Errorf("%s: %w", fieldName, err)
	}
	if redactor != nil {
		redactor.AddSensitive(val)
	}
	fv.SetString(val)
	return nil
}
