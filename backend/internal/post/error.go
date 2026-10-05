package post

import (
	"errors"
	"fmt"
)

// Sentinel errors mapped to HTTP responses by the handler.
var (
	// ErrPostNotFound means the post does not exist.
	ErrPostNotFound = errors.New("post not found")
	// ErrForbidden means the acting user may not modify this post.
	ErrForbidden = errors.New("forbidden")
	// ErrGuestViewDenied means guest viewing is disabled and the caller is anonymous.
	ErrGuestViewDenied = errors.New("guest view denied")
	// ErrBadRequest means the request is invalid for the current state.
	ErrBadRequest = errors.New("bad request")
	// ErrAuthorNotFound means the requested post author does not exist.
	ErrAuthorNotFound = errors.New("author not found")
	// ErrInvalidStatus means the requested post status is not a value the
	// author-facing endpoints accept (unknown value, or the moderation-only
	// `rejected` state).
	ErrInvalidStatus = errors.New("invalid post status")
	// ErrDuplicateName means a category or tag with the same name already exists.
	ErrDuplicateName = errors.New("duplicate name")
	// ErrCategoryNotFound means the category does not exist.
	ErrCategoryNotFound = errors.New("category not found")
	// ErrTagNotFound means the tag does not exist.
	ErrTagNotFound = errors.New("tag not found")
)

// InUseError means a category or tag is still referenced by posts; Count
// carries the number of referencing posts so callers can render it.
type InUseError struct {
	Count int64
}

// Error renders the in-use reason.
func (e *InUseError) Error() string {
	return fmt.Sprintf("in use by %d posts", e.Count)
}
