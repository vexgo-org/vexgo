package model

import (
	"testing"
	"unicode/utf8"
)

// TruncateRunes must cut on character boundaries: the previous byte-slice
// truncation produced invalid UTF-8, which strict databases (MySQL/PostgreSQL)
// reject, silently losing the write that carried it.
func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{name: "shorter than limit is returned unchanged", in: "abc", n: 5, want: "abc"},
		{name: "exactly at limit is returned unchanged", in: "abc", n: 3, want: "abc"},
		{name: "ascii is cut at the limit", in: "abcdef", n: 3, want: "abc"},
		{name: "empty input", in: "", n: 3, want: ""},
		{name: "zero limit yields empty", in: "abc", n: 0, want: ""},
		{name: "negative limit yields empty instead of panicking", in: "abc", n: -1, want: ""},
		{name: "cjk counts characters, not bytes", in: "中文测试", n: 2, want: "中文"},
		{name: "cjk at the limit is returned unchanged", in: "中文", n: 2, want: "中文"},
		{name: "emoji is not split", in: "🙂🙂🙂", n: 2, want: "🙂🙂"},
		{name: "mixed width", in: "ab中文", n: 3, want: "ab中"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TruncateRunes(tc.in, tc.n)
			if got != tc.want {
				t.Errorf("TruncateRunes(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("TruncateRunes(%q, %d) = %q, which is not valid UTF-8", tc.in, tc.n, got)
			}
		})
	}
}

// The result never exceeds the rune budget, whatever the limit or input width.
func TestTruncateRunesCapsRuneCount(t *testing.T) {
	inputs := []string{"静夜思，床前明月光", "abcdefghij", "🙂🙂🙂🙂🙂"}

	for _, limit := range []int{1, 2, 5, 200} {
		for _, in := range inputs {
			got := TruncateRunes(in, limit)
			if n := utf8.RuneCountInString(got); n > limit {
				t.Errorf("TruncateRunes(%q, %d) returned %d runes", in, limit, n)
			}
		}
	}
}
