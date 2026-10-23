// Package stringlist converts a list of strings to the list type of format data.
package stringlist

// Any returns values as the []any that a model.Data list holds.
func Any(values []string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}
