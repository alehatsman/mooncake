package expression

import (
	"fmt"
	"math"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

// regexCache caches compiled regex patterns for performance
var regexCache sync.Map

// StringFunctions returns string manipulation functions
//
//nolint:gocyclo // High complexity due to comprehensive error checking for all string functions
func StringFunctions() map[string]interface{} {
	return map[string]interface{}{
		// starts_with checks if string starts with prefix
		"starts_with": func(params ...interface{}) (interface{}, error) {
			if len(params) != 2 {
				return false, fmt.Errorf("starts_with requires 2 arguments, got %d", len(params))
			}
			str, ok1 := params[0].(string)
			prefix, ok2 := params[1].(string)
			if !ok1 || !ok2 {
				return false, fmt.Errorf("starts_with requires string arguments")
			}
			return strings.HasPrefix(str, prefix), nil
		},

		// ends_with checks if string ends with suffix
		"ends_with": func(params ...interface{}) (interface{}, error) {
			if len(params) != 2 {
				return false, fmt.Errorf("ends_with requires 2 arguments, got %d", len(params))
			}
			str, ok1 := params[0].(string)
			suffix, ok2 := params[1].(string)
			if !ok1 || !ok2 {
				return false, fmt.Errorf("ends_with requires string arguments")
			}
			return strings.HasSuffix(str, suffix), nil
		},

		// lower converts string to lowercase
		"lower": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return "", fmt.Errorf("lower requires 1 argument, got %d", len(params))
			}
			str, ok := params[0].(string)
			if !ok {
				return "", fmt.Errorf("lower requires a string argument")
			}
			return strings.ToLower(str), nil
		},

		// upper converts string to uppercase
		"upper": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return "", fmt.Errorf("upper requires 1 argument, got %d", len(params))
			}
			str, ok := params[0].(string)
			if !ok {
				return "", fmt.Errorf("upper requires a string argument")
			}
			return strings.ToUpper(str), nil
		},

		// trim removes leading and trailing whitespace
		"trim": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return "", fmt.Errorf("trim requires 1 argument, got %d", len(params))
			}
			str, ok := params[0].(string)
			if !ok {
				return "", fmt.Errorf("trim requires a string argument")
			}
			return strings.TrimSpace(str), nil
		},

		// split splits string by separator
		"split": func(params ...interface{}) (interface{}, error) {
			if len(params) != 2 {
				return []string{}, fmt.Errorf("split requires 2 arguments, got %d", len(params))
			}
			str, ok1 := params[0].(string)
			sep, ok2 := params[1].(string)
			if !ok1 || !ok2 {
				return []string{}, fmt.Errorf("split requires string arguments")
			}
			return strings.Split(str, sep), nil
		},

		// join joins array elements with separator
		"join": func(params ...interface{}) (interface{}, error) {
			if len(params) != 2 {
				return "", fmt.Errorf("join requires 2 arguments, got %d", len(params))
			}
			sep, ok := params[1].(string)
			if !ok {
				return "", fmt.Errorf("join requires separator to be a string")
			}

			// Handle array
			var arr []interface{}
			switch v := params[0].(type) {
			case []interface{}:
				arr = v
			case []string:
				arr = make([]interface{}, len(v))
				for i, s := range v {
					arr[i] = s
				}
			case []int:
				arr = make([]interface{}, len(v))
				for i, n := range v {
					arr[i] = n
				}
			default:
				return "", fmt.Errorf("join requires an array as first argument")
			}

			strs := make([]string, len(arr))
			for i, v := range arr {
				strs[i] = fmt.Sprintf("%v", v)
			}
			return strings.Join(strs, sep), nil
		},

		// replace replaces all occurrences of old with new
		"replace": func(params ...interface{}) (interface{}, error) {
			if len(params) != 3 {
				return "", fmt.Errorf("replace requires 3 arguments, got %d", len(params))
			}
			str, ok1 := params[0].(string)
			old, ok2 := params[1].(string)
			newStr, ok3 := params[2].(string)
			if !ok1 || !ok2 || !ok3 {
				return "", fmt.Errorf("replace requires string arguments")
			}
			return strings.ReplaceAll(str, old, newStr), nil
		},

		// regex_match checks if string matches regex pattern
		"regex_match": func(params ...interface{}) (interface{}, error) {
			if len(params) != 2 {
				return false, fmt.Errorf("regex_match requires 2 arguments, got %d", len(params))
			}
			str, ok1 := params[0].(string)
			pattern, ok2 := params[1].(string)
			if !ok1 || !ok2 {
				return false, fmt.Errorf("regex_match requires string arguments")
			}

			// Try to get compiled regex from cache
			var re *regexp.Regexp
			if cached, ok := regexCache.Load(pattern); ok {
				re, ok = cached.(*regexp.Regexp)
				if !ok {
					return false, fmt.Errorf("invalid cached regex pattern")
				}
			} else {
				// Compile and cache the pattern
				compiled, err := regexp.Compile(pattern)
				if err != nil {
					return false, fmt.Errorf("invalid regex pattern: %w", err)
				}
				re = compiled
				regexCache.Store(pattern, re)
			}

			return re.MatchString(str), nil
		},
	}
}

// MathFunctions returns mathematical functions
func MathFunctions() map[string]interface{} {
	return map[string]interface{}{
		// min returns minimum of two numbers
		"min": func(params ...interface{}) (interface{}, error) {
			if len(params) != 2 {
				return 0.0, fmt.Errorf("min requires 2 arguments, got %d", len(params))
			}
			a, err1 := toFloat64(params[0])
			b, err2 := toFloat64(params[1])
			if err1 != nil || err2 != nil {
				return 0.0, fmt.Errorf("min requires numeric arguments")
			}
			return math.Min(a, b), nil
		},

		// max returns maximum of two numbers
		"max": func(params ...interface{}) (interface{}, error) {
			if len(params) != 2 {
				return 0.0, fmt.Errorf("max requires 2 arguments, got %d", len(params))
			}
			a, err1 := toFloat64(params[0])
			b, err2 := toFloat64(params[1])
			if err1 != nil || err2 != nil {
				return 0.0, fmt.Errorf("max requires numeric arguments")
			}
			return math.Max(a, b), nil
		},

		// abs returns absolute value
		"abs": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return 0.0, fmt.Errorf("abs requires 1 argument, got %d", len(params))
			}
			n, err := toFloat64(params[0])
			if err != nil {
				return 0.0, fmt.Errorf("abs requires a numeric argument")
			}
			return math.Abs(n), nil
		},

		// floor returns floor value
		"floor": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return 0.0, fmt.Errorf("floor requires 1 argument, got %d", len(params))
			}
			n, err := toFloat64(params[0])
			if err != nil {
				return 0.0, fmt.Errorf("floor requires a numeric argument")
			}
			return math.Floor(n), nil
		},

		// ceil returns ceiling value
		"ceil": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return 0.0, fmt.Errorf("ceil requires 1 argument, got %d", len(params))
			}
			n, err := toFloat64(params[0])
			if err != nil {
				return 0.0, fmt.Errorf("ceil requires a numeric argument")
			}
			return math.Ceil(n), nil
		},

		// round rounds to nearest integer
		"round": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return 0.0, fmt.Errorf("round requires 1 argument, got %d", len(params))
			}
			n, err := toFloat64(params[0])
			if err != nil {
				return 0.0, fmt.Errorf("round requires a numeric argument")
			}
			return math.Round(n), nil
		},

		// pow returns a^b
		"pow": func(params ...interface{}) (interface{}, error) {
			if len(params) != 2 {
				return 0.0, fmt.Errorf("pow requires 2 arguments, got %d", len(params))
			}
			a, err1 := toFloat64(params[0])
			b, err2 := toFloat64(params[1])
			if err1 != nil || err2 != nil {
				return 0.0, fmt.Errorf("pow requires numeric arguments")
			}
			return math.Pow(a, b), nil
		},

		// sqrt returns square root
		"sqrt": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return 0.0, fmt.Errorf("sqrt requires 1 argument, got %d", len(params))
			}
			n, err := toFloat64(params[0])
			if err != nil {
				return 0.0, fmt.Errorf("sqrt requires a numeric argument")
			}
			return math.Sqrt(n), nil
		},
	}
}

// toFloat64 converts various numeric types to float64
func toFloat64(v interface{}) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case float32:
		return float64(n), nil
	case int:
		return float64(n), nil
	case int8:
		return float64(n), nil
	case int16:
		return float64(n), nil
	case int32:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case uint:
		return float64(n), nil
	case uint8:
		return float64(n), nil
	case uint16:
		return float64(n), nil
	case uint32:
		return float64(n), nil
	case uint64:
		return float64(n), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

// CollectionFunctions returns array/collection functions
func CollectionFunctions() map[string]interface{} {
	return map[string]interface{}{
		// len returns length of string, array, or map
		"len": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return 0, fmt.Errorf("len requires 1 argument, got %d", len(params))
			}
			v := params[0]
			if v == nil {
				return 0, nil
			}
			val := reflect.ValueOf(v)
			switch val.Kind() {
			case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
				return val.Len(), nil
			default:
				return 0, nil
			}
		},

		// includes checks if item is in array
		"includes": func(params ...interface{}) (interface{}, error) {
			if len(params) != 2 {
				return false, fmt.Errorf("includes requires 2 arguments, got %d", len(params))
			}
			item := params[0]
			arr := params[1]
			if arr == nil {
				return false, nil
			}
			val := reflect.ValueOf(arr)
			if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
				return false, nil
			}
			for i := 0; i < val.Len(); i++ {
				if reflect.DeepEqual(item, val.Index(i).Interface()) {
					return true, nil
				}
			}
			return false, nil
		},

		// empty checks if value is empty (nil, empty string, empty array, zero)
		"empty": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return true, fmt.Errorf("empty requires 1 argument, got %d", len(params))
			}
			v := params[0]
			if v == nil {
				return true, nil
			}
			val := reflect.ValueOf(v)
			switch val.Kind() {
			case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
				return val.Len() == 0, nil
			case reflect.Bool:
				return !val.Bool(), nil
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				return val.Int() == 0, nil
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				return val.Uint() == 0, nil
			case reflect.Float32, reflect.Float64:
				return val.Float() == 0, nil
			default:
				return false, nil
			}
		},

		// first returns first element of array
		"first": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return nil, fmt.Errorf("first requires 1 argument, got %d", len(params))
			}
			arr := params[0]
			if arr == nil {
				return nil, nil
			}
			val := reflect.ValueOf(arr)
			if (val.Kind() != reflect.Slice && val.Kind() != reflect.Array) || val.Len() == 0 {
				return nil, nil
			}
			return val.Index(0).Interface(), nil
		},

		// last returns last element of array
		"last": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return nil, fmt.Errorf("last requires 1 argument, got %d", len(params))
			}
			arr := params[0]
			if arr == nil {
				return nil, nil
			}
			val := reflect.ValueOf(arr)
			if (val.Kind() != reflect.Slice && val.Kind() != reflect.Array) || val.Len() == 0 {
				return nil, nil
			}
			return val.Index(val.Len() - 1).Interface(), nil
		},
	}
}

// TypeFunctions returns type checking functions
func TypeFunctions() map[string]interface{} {
	return map[string]interface{}{
		// is_string checks if value is a string
		"is_string": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return false, fmt.Errorf("is_string requires 1 argument, got %d", len(params))
			}
			_, ok := params[0].(string)
			return ok, nil
		},

		// is_number checks if value is a number
		"is_number": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return false, fmt.Errorf("is_number requires 1 argument, got %d", len(params))
			}
			v := params[0]
			if v == nil {
				return false, nil
			}
			val := reflect.ValueOf(v)
			kind := val.Kind()
			return kind >= reflect.Int && kind <= reflect.Float64, nil
		},

		// is_bool checks if value is a boolean
		"is_bool": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return false, fmt.Errorf("is_bool requires 1 argument, got %d", len(params))
			}
			_, ok := params[0].(bool)
			return ok, nil
		},

		// is_array checks if value is an array or slice
		"is_array": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return false, fmt.Errorf("is_array requires 1 argument, got %d", len(params))
			}
			v := params[0]
			if v == nil {
				return false, nil
			}
			val := reflect.ValueOf(v)
			return val.Kind() == reflect.Slice || val.Kind() == reflect.Array, nil
		},

		// is_map checks if value is a map
		"is_map": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return false, fmt.Errorf("is_map requires 1 argument, got %d", len(params))
			}
			v := params[0]
			if v == nil {
				return false, nil
			}
			return reflect.ValueOf(v).Kind() == reflect.Map, nil
		},

		// is_defined checks if variable is not nil
		"is_defined": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return false, fmt.Errorf("is_defined requires 1 argument, got %d", len(params))
			}
			return params[0] != nil, nil
		},
	}
}

// UtilityFunctions returns utility functions
func UtilityFunctions() map[string]interface{} {
	return map[string]interface{}{
		// default returns defaultValue if v is nil or empty
		"default": func(params ...interface{}) (interface{}, error) {
			if len(params) != 2 {
				return nil, fmt.Errorf("default requires 2 arguments, got %d", len(params))
			}
			v := params[0]
			defaultValue := params[1]
			if v == nil {
				return defaultValue, nil
			}
			// Check if empty string
			if str, ok := v.(string); ok && str == "" {
				return defaultValue, nil
			}
			// Check if empty array/slice
			val := reflect.ValueOf(v)
			if (val.Kind() == reflect.Slice || val.Kind() == reflect.Array) && val.Len() == 0 {
				return defaultValue, nil
			}
			return v, nil
		},

		// env gets environment variable
		"env": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return "", fmt.Errorf("env requires 1 argument, got %d", len(params))
			}
			name, ok := params[0].(string)
			if !ok {
				return "", fmt.Errorf("env requires a string argument")
			}
			return os.Getenv(name), nil
		},

		// has_env checks if environment variable exists
		"has_env": func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return false, fmt.Errorf("has_env requires 1 argument, got %d", len(params))
			}
			name, ok := params[0].(string)
			if !ok {
				return false, fmt.Errorf("has_env requires a string argument")
			}
			_, exists := os.LookupEnv(name)
			return exists, nil
		},

		// coalesce returns first non-nil value
		"coalesce": func(params ...interface{}) (interface{}, error) {
			for _, v := range params {
				if v != nil {
					if str, ok := v.(string); ok && str != "" {
						return v, nil
					} else if !ok {
						return v, nil
					}
				}
			}
			return nil, nil
		},

		// ternary returns trueValue if condition is true, else falseValue
		"ternary": func(params ...interface{}) (interface{}, error) {
			if len(params) != 3 {
				return nil, fmt.Errorf("ternary requires 3 arguments, got %d", len(params))
			}
			condition, ok := params[0].(bool)
			if !ok {
				return nil, fmt.Errorf("ternary requires boolean as first argument")
			}
			if condition {
				return params[1], nil
			}
			return params[2], nil
		},
	}
}

// AllFunctions returns all custom functions combined
func AllFunctions() map[string]interface{} {
	all := make(map[string]interface{})

	// Merge all function maps
	for name, fn := range StringFunctions() {
		all[name] = fn
	}
	for name, fn := range MathFunctions() {
		all[name] = fn
	}
	for name, fn := range CollectionFunctions() {
		all[name] = fn
	}
	for name, fn := range TypeFunctions() {
		all[name] = fn
	}
	for name, fn := range UtilityFunctions() {
		all[name] = fn
	}

	return all
}
