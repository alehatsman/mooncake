package utils

// MergeVariables merges two variable maps, with override taking precedence.
// Returns a new map containing all keys from both maps. When a key exists in
// both maps, the value from override is used.
func MergeVariables(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
}
