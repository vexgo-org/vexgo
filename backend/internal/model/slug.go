package model

import (
	"errors"
	"regexp"
	"strings"
)

// Slug validation errors.
var (
	ErrEmptySlug   = errors.New("slug cannot be empty")
	ErrSlugTooLong = errors.New("slug exceeds maximum length")
	ErrInvalidSlug = errors.New("slug must contain only lowercase letters, digits, and hyphens (no leading, trailing, or consecutive hyphens)")
	ErrSlugTaken   = errors.New("slug is already taken by another post")
)

// SlugPattern is the regex for valid post slugs: lowercase letters, digits,
// and hyphens; no leading, trailing, or consecutive hyphens; max length 200.
var SlugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// MaxSlugLength is the maximum allowed slug length.
const MaxSlugLength = 200

// ValidateSlug checks whether a slug conforms to the URL-safe pattern and
// length limit. The slug must contain at least one letter so that pure-numeric
// values (which would collide with old numeric URLs) are rejected.
func ValidateSlug(slug string) error {
	if slug == "" {
		return ErrEmptySlug
	}
	if len(slug) > MaxSlugLength {
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

func containsLetter(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

// SlugFromTitle generates a URL-safe slug from a post title. Only ASCII
// letters and digits are preserved; spaces and separators become hyphens.
// When the result has no valid characters, it returns "" so the caller can
// substitute a fallback.
func SlugFromTitle(title string) string {
	s := strings.ToLower(title)

	var result strings.Builder
	result.Grow(len(s))

	for _, r := range s {
		switch {
		case r == '-' || r == '_' || r == ' ':
			result.WriteByte('-')
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			result.WriteRune(r)
		case r >= '0' && r <= '9':
			result.WriteRune(r)
		default:
			// Skip non-ASCII and special characters.
		}
	}

	s = result.String()

	// Collapse consecutive hyphens and trim leading/trailing hyphens.
	s = collapseHyphens(s)
	s = strings.Trim(s, "-")

	if len(s) > MaxSlugLength {
		s = s[:MaxSlugLength]
		s = strings.TrimRight(s, "-")
	}

	return s
}

func collapseHyphens(s string) string {
	result := make([]byte, 0, len(s))
	prev := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '-' {
			if prev {
				continue
			}
			prev = true
		} else {
			prev = false
		}
		result = append(result, ch)
	}
	return string(result)
}
