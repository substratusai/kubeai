package v1

// firstNChars returns the first n characters of a string.
// This function is needed because Go's string indexing is based on bytes, not runes.
func firstNChars(s string, n int) string {
	runes := []rune(s)
	return string(runes[:min(n, len(runes))])
}

// Ptr is a helper function for creating an inline pointer to a constant.
func Ptr[T any](v T) *T {
	return &v
}
