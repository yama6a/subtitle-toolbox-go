package formatdata

// Strings returns values as the []any that a model.Data list holds.
func Strings(values []string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}
