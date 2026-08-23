package model

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Slug validation errors.
var (
	ErrEmptySlug   = errors.New("slug cannot be empty")
	ErrSlugTooLong = errors.New("slug exceeds maximum length")
	ErrInvalidSlug = errors.New("slug must contain Unicode letters, numbers, and hyphens only, with no leading, trailing, or consecutive hyphens")
	ErrSlugTaken   = errors.New("slug is already taken by another post")
)

// SlugPattern matches Unicode letters, numbers, combining marks, and hyphens.
// Each segment must start with a letter or number.
var SlugPattern = regexp.MustCompile(
	`^[\p{L}\p{N}][\p{L}\p{N}\p{M}]*(?:-[\p{L}\p{N}][\p{L}\p{N}\p{M}]*)*$`,
)

// MaxSlugLength is the maximum allowed slug length in Unicode characters.
const MaxSlugLength = 200

// ValidateSlug checks whether a slug conforms to the Unicode-safe pattern and
// length limit. The slug must contain at least one Unicode letter so that
// pure-numeric values, which may collide with old numeric URLs, are rejected.
func ValidateSlug(slug string) error {
	if slug == "" {
		return ErrEmptySlug
	}

	if utf8.RuneCountInString(slug) > MaxSlugLength {
		return ErrSlugTooLong
	}

	if !SlugPattern.MatchString(slug) {
		return ErrInvalidSlug
	}

	if !containsLetter(slug) {
		return ErrInvalidSlug
	}

	return nil
}

// containsLetter reports whether the string contains at least one Unicode letter.
func containsLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}

	return false
}

// SlugFromTitle generates a Unicode-safe slug from a post title.
// Letters and numbers from all languages are preserved.
// Spaces, underscores, and Unicode dash characters become hyphens.
// When the result has no valid characters, an empty string is returned so
// the caller can provide a fallback.
func SlugFromTitle(title string) string {
	title = strings.ToLower(title)

	var result strings.Builder
	result.Grow(len(title))

	pendingHyphen := false
	segmentHasBase := false

	for _, r := range title {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			if pendingHyphen && result.Len() > 0 {
				result.WriteByte('-')
			}

			result.WriteRune(r)
			pendingHyphen = false
			segmentHasBase = true

		case unicode.IsMark(r):
			// Keep combining marks only after a letter or number.
			if segmentHasBase && !pendingHyphen {
				result.WriteRune(r)
			}

		case r == '_' || unicode.IsSpace(r) || unicode.Is(unicode.Pd, r):
			if result.Len() > 0 {
				pendingHyphen = true
				segmentHasBase = false
			}

		default:
			// Skip unsupported punctuation and symbols.
		}
	}

	s := result.String()

	runes := []rune(s)
	if len(runes) > MaxSlugLength {
		runes = runes[:MaxSlugLength]
		s = strings.TrimRight(string(runes), "-")
	}

	return s
}
