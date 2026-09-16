package model

// TruncateRunes shortens s to at most n runes without splitting a multi-byte
// character. Slicing by byte instead would cut a CJK or emoji character in
// half and produce invalid UTF-8, which strict databases reject on insert.
//
// A non-positive n yields the empty string rather than panicking on the slice.
// Callers that need to know whether truncation happened compare
// utf8.RuneCountInString(s) against n themselves.
func TruncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
