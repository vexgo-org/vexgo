package post

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/vexgo-org/vexgo/backend/internal/auth"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// ListQuery carries the acting user, pagination and filters for List.
type ListQuery struct {
	UserRole string
	UserID   uint
	Page     int
	Limit    int
	Category string
	Search   string
}

// List returns the paginated post list with role-based visibility, filters,
// and per-post like/comment counts.
func (s *Service) List(ctx context.Context, q ListQuery) ([]model.Post, int64, error) {
	// If not logged in and guest viewing is not allowed, deny access
	if q.UserRole == "" && !s.allowGuestView(ctx) {
		return nil, 0, ErrGuestViewDenied
	}

	posts, total, err := s.repo.List(ctx, q.UserRole, q.UserID, ListFilter{
		Page: q.Page, Limit: q.Limit, CategoryID: q.Category, Search: q.Search,
	})
	if err != nil {
		return nil, 0, err
	}

	s.populateCounts(ctx, posts, q.UserID)

	// Apply privacy filtering to author information
	for i := range posts {
		if !model.IsAdmin(q.UserRole) && posts[i].AuthorID != q.UserID {
			auth.FilterUserByPrivacy(&posts[i].Author, q.UserID, q.UserRole)
		}
	}

	return posts, total, nil
}

// Get returns a single post by numeric ID with privacy filtering, view-count
// increment and like/comment counts. Used by internal operations (edit,
// delete, likes, moderation).
func (s *Service) Get(ctx context.Context, id, currentUserRole string, currentUserID uint) (*model.Post, error) {
	post, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("post load: %w", err)
	}

	return s.enrichPost(ctx, post, currentUserRole, currentUserID)
}

// GetBySlug returns a single post by slug with privacy filtering, view-count
// increment and like/comment counts. Used by the public read route.
func (s *Service) GetBySlug(ctx context.Context, slug, currentUserRole string, currentUserID uint) (*model.Post, error) {
	post, err := s.repo.FindBySlug(ctx, slug)
	if err != nil {
		return nil, ErrPostNotFound
	}

	return s.enrichPost(ctx, post, currentUserRole, currentUserID)
}

// enrichPost fills like/comment counts, view count and privacy filtering on a
// post that was just loaded from the database.
func (s *Service) enrichPost(ctx context.Context, post *model.Post, currentUserRole string, currentUserID uint) (*model.Post, error) {
	// If not logged in and guest viewing is not allowed, return 403
	if currentUserRole == "" && !s.allowGuestView(ctx) {
		return nil, ErrGuestViewDenied
	}

	if !model.IsAdmin(currentUserRole) && post.AuthorID != currentUserID {
		auth.FilterUserByPrivacy(&post.Author, currentUserID, currentUserRole)
	}

	// Increment view count (best-effort)
	if err := s.repo.IncrementViewCount(ctx, post.ID); err != nil {
		slog.Warn("failed to increment view count", "err", err)
	}

	// Fill likes count and current logged-in user's like status
	count, _ := s.repo.CountLikes(ctx, post.ID)
	post.LikesCount = int(count)
	post.IsLiked = false
	if currentUserID != 0 {
		if _, err := s.repo.FindLike(ctx, post.ID, currentUserID); err == nil {
			post.IsLiked = true
		}
	}

	// Fill comments count
	ccount, _ := s.repo.CountComments(ctx, post.ID)
	post.CommentsCount = int(ccount)

	return post, nil
}

// CreateRequest carries the fields accepted when creating a post.
type CreateRequest struct {
	Slug       string
	Title      string
	Content    string
	Category   any
	Tags       []string
	Excerpt    string
	CoverImage string
	Status     model.PostStatus
}

// Create creates a post, deriving the initial status from the user's role.
func (s *Service) Create(ctx context.Context, userRole string, userID uint, req CreateRequest) (*model.Post, error) {
	// Check if user has permission to create posts
	if userRole == "" || userRole == model.RoleGuest {
		return nil, ErrForbidden
	}

	// Validate slug
	if err := model.ValidateSlug(req.Slug); err != nil {
		return nil, fmt.Errorf("slug: %w", err)
	}

	// Check for duplicate slug
	if _, err := s.repo.FindBySlug(ctx, req.Slug); err == nil {
		return nil, fmt.Errorf("slug: %w", model.ErrSlugTaken)
	}

	// Convert category to string regardless of number or string type
	var catStr string
	switch v := req.Category.(type) {
	case string:
		catStr = v
	case float64:
		catStr = strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		catStr = strconv.Itoa(v)
	case int64:
		catStr = strconv.FormatInt(v, 10)
	default:
		catStr = fmt.Sprintf("%v", v)
	}

	// Determine initial post status based on user role
	initialStatus := model.PostStatus(req.Status)
	if initialStatus == "" {
		switch userRole {
		case model.RoleContributor:
			initialStatus = model.PostStatusPending
		case model.RoleAuthor, model.RoleAdmin, model.RoleSuperAdmin:
			initialStatus = model.PostStatusPublished
		default:
			initialStatus = model.PostStatusDraft
		}
	}

	post := model.Post{
		Slug:       req.Slug,
		Title:      req.Title,
		Content:    req.Content,
		Category:   catStr,
		Excerpt:    req.Excerpt,
		CoverImage: req.CoverImage,
		Status:     initialStatus,
		AuthorID:   userID,
	}

	if len(req.Tags) > 0 {
		tags, err := s.resolveTags(ctx, req.Tags)
		if err != nil {
			return nil, fmt.Errorf("resolve tags: %w", err)
		}
		post.Tags = tags
	}

	if err := s.repo.Create(ctx, &post); err != nil {
		return nil, err
	}

	return &post, nil
}

// UpdateRequest carries the fields accepted when updating a post.
type UpdateRequest struct {
	Slug       string
	Title      string
	Content    string
	Category   any
	Tags       []string
	Excerpt    string
	CoverImage string
	Status     model.PostStatus
}

// Update modifies a post when the acting user is its author or an admin.
func (s *Service) Update(ctx context.Context, id string, userID uint, req UpdateRequest) (*model.Post, error) {
	post, err := s.repo.FindByIDPreloadTags(ctx, id)
	if err != nil {
		return nil, ErrPostNotFound
	}

	// Permission check
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, ErrForbidden
	}
	if !model.IsAdmin(user.Role) && post.AuthorID != userID {
		return nil, ErrForbidden
	}

	if req.Slug != "" && req.Slug != post.Slug {
		if err := model.ValidateSlug(req.Slug); err != nil {
			return nil, fmt.Errorf("slug: %w", err)
		}
		// Allow the post to keep its own slug, reject duplicates with others.
		if _, err := s.repo.FindBySlugExcludeID(ctx, req.Slug, post.ID); err == nil {
			return nil, fmt.Errorf("slug: %w", model.ErrSlugTaken)
		}
		post.Slug = req.Slug
	}

	if req.Title != "" {
		post.Title = req.Title
	}
	if req.Content != "" {
		post.Content = req.Content
	}
	if req.Category != nil {
		switch v := req.Category.(type) {
		case string:
			post.Category = v
		case float64:
			post.Category = strconv.FormatFloat(v, 'f', -1, 64)
		case int:
			post.Category = strconv.Itoa(v)
		case int64:
			post.Category = strconv.FormatInt(v, 10)
		default:
			post.Category = fmt.Sprintf("%v", v)
		}
	}
	if req.Excerpt != "" {
		post.Excerpt = req.Excerpt
	}
	if req.CoverImage != "" {
		post.CoverImage = req.CoverImage
	}
	if req.Status != "" {
		post.Status = model.PostStatus(req.Status)
	}

	if len(req.Tags) > 0 {
		tags, err := s.resolveTags(ctx, req.Tags)
		if err != nil {
			return nil, fmt.Errorf("resolve tags: %w", err)
		}
		if err := s.repo.ReplaceTagsAssociation(ctx, post, tags); err != nil {
			return nil, fmt.Errorf("replace tags: %w", err)
		}
		post.Tags = tags
	}

	if err := s.repo.Save(ctx, post); err != nil {
		return nil, fmt.Errorf("save post: %w", err)
	}
	return post, nil
}

// Delete removes a post (author or admin only), cleaning up its files,
// comments, likes and tag associations.
func (s *Service) Delete(ctx context.Context, id string, userID uint) error {
	post, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return ErrPostNotFound
	}

	// Permission check
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return ErrForbidden
	}
	if !model.IsAdmin(user.Role) && post.AuthorID != userID {
		return ErrForbidden
	}

	// Collect and delete image files (cover + content images)
	var imagesToDelete []string
	if post.CoverImage != "" {
		imagesToDelete = append(imagesToDelete, post.CoverImage)
	}
	imagesToDelete = append(imagesToDelete, extractImageURLs(post.Content)...)

	uniqueImages := make(map[string]bool)
	for _, url := range imagesToDelete {
		uniqueImages[url] = true
	}
	for url := range uniqueImages {
		if err := s.files.Delete(url); err != nil {
			slog.Warn("failed to delete image",
				"url", url,
				"err", err,
			)
		}
	}

	// Delete comments associated with this post
	if err := s.repo.DeleteCommentsByPostID(ctx, post.ID); err != nil {
		return fmt.Errorf("delete comments: %w", err)
	}

	// Delete likes associated with this post
	if err := s.repo.DeleteLikesByPostID(ctx, post.ID); err != nil {
		return fmt.Errorf("delete likes: %w", err)
	}

	// Clear the many-to-many tags relationship
	if err := s.repo.ClearTagsAssociation(ctx, post); err != nil {
		return fmt.Errorf("clear tags: %w", err)
	}

	// Finally delete the post itself
	if err := s.repo.Delete(ctx, post); err != nil {
		return fmt.Errorf("delete post: %w", err)
	}

	return nil
}

// MyPostsQuery carries the pagination and status filter for MyPosts.
type MyPostsQuery struct {
	UserID uint
	Page   int
	Limit  int
	Status string
}

// MyPosts returns the current user's own posts (excluding rejected).
func (s *Service) MyPosts(ctx context.Context, q MyPostsQuery) ([]model.Post, int64, error) {
	posts, total, err := s.repo.MyPosts(ctx, q.UserID, ListFilter{Page: q.Page, Limit: q.Limit, Status: q.Status})
	if err != nil {
		return nil, 0, err
	}

	s.populateCounts(ctx, posts, q.UserID)
	return posts, total, nil
}

// DraftsQuery carries the acting user and pagination for Drafts.
type DraftsQuery struct {
	UserRole string
	UserID   uint
	Page     int
	Limit    int
}

// Drafts returns draft posts; admins see all drafts, other users only their own.
func (s *Service) Drafts(ctx context.Context, q DraftsQuery) ([]model.Post, int64, error) {
	posts, total, err := s.repo.Drafts(ctx, q.UserRole, q.UserID, ListFilter{Page: q.Page, Limit: q.Limit})
	if err != nil {
		return nil, 0, err
	}

	s.populateCounts(ctx, posts, q.UserID)
	return posts, total, nil
}

// UserPostsQuery carries the target user, acting user and pagination.
type UserPostsQuery struct {
	UserIDStr       string
	CurrentUserRole string
	CurrentUserID   uint
	Page            int
	Limit           int
}

// UserPosts returns the posts of a specific user with role-based visibility.
func (s *Service) UserPosts(ctx context.Context, q UserPostsQuery) ([]model.Post, int64, error) {
	uid, err := strconv.Atoi(q.UserIDStr)
	if err != nil {
		return nil, 0, ErrBadRequest
	}

	if q.CurrentUserRole == "" && !s.allowGuestView(ctx) {
		return []model.Post{}, 0, nil
	}

	posts, total, err := s.repo.UserPosts(
		ctx, uint(uid), q.CurrentUserRole, q.CurrentUserID,
		ListFilter{Page: q.Page, Limit: q.Limit},
	)
	if err != nil {
		return nil, 0, err
	}

	s.populateCounts(ctx, posts, q.CurrentUserID)

	// Apply privacy filtering to author information
	for i := range posts {
		if !model.IsAdmin(q.CurrentUserRole) && uint(uid) != q.CurrentUserID {
			auth.FilterUserByPrivacy(&posts[i].Author, q.CurrentUserID, q.CurrentUserRole)
		}
	}

	return posts, total, nil
}

// Popular returns the top posts by likes*5 + views, limited to published posts.
func (s *Service) Popular(ctx context.Context, userRole string, limit int) ([]model.Post, error) {
	if userRole == "" && !s.allowGuestView(ctx) {
		return nil, ErrGuestViewDenied
	}

	posts, err := s.repo.Popular(ctx)
	if err != nil {
		return nil, err
	}

	// Batch fetch like counts to avoid N+1 queries
	ids := make([]uint, len(posts))
	for i := range posts {
		ids[i] = posts[i].ID
	}
	counts, err := s.repo.BatchCountLikesByPostIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range posts {
		posts[i].LikesCount = int(counts[posts[i].ID])
	}

	// Sort by the sum of likes count multiplied by 5 plus view count
	sort.Slice(posts, func(i, j int) bool {
		scoreI := posts[i].LikesCount*5 + posts[i].ViewCount
		scoreJ := posts[j].LikesCount*5 + posts[j].ViewCount
		return scoreI > scoreJ
	})

	// Limit the number of returned items
	if limit < len(posts) {
		posts = posts[:limit]
	}

	return posts, nil
}

// Latest returns the most recent published posts.
func (s *Service) Latest(ctx context.Context, userRole string, limit int) ([]model.Post, error) {
	if userRole == "" && !s.allowGuestView(ctx) {
		return []model.Post{}, nil
	}

	return s.repo.Latest(ctx, limit)
}

// populateCounts fills like/comment counts and the current user's like status
// for a batch of posts using batch queries to avoid N+1.
func (s *Service) populateCounts(ctx context.Context, posts []model.Post, userID uint) {
	if len(posts) == 0 {
		return
	}

	postIDs := make([]uint, len(posts))
	for i := range posts {
		postIDs[i] = posts[i].ID
	}

	likesCounts, _ := s.repo.BatchCountLikesByPostIDs(ctx, postIDs)
	commentsCounts, _ := s.repo.BatchCountCommentsByPostIDs(ctx, postIDs)
	var likedPosts map[uint]bool
	if userID != 0 {
		likedPosts, _ = s.repo.BatchFindLikedPostIDs(ctx, postIDs, userID)
	}

	for i := range posts {
		posts[i].LikesCount = int(likesCounts[posts[i].ID])
		posts[i].CommentsCount = int(commentsCounts[posts[i].ID])
		posts[i].IsLiked = likedPosts[posts[i].ID]
	}
}

// extractImageURLs returns all image URLs referenced in HTML or Markdown content.
func extractImageURLs(content string) []string {
	var urls []string

	// 1. Match src attribute of <img> tags (HTML)
	reImg := regexp.MustCompile(`<img[^>]+src=["']([^"']+)["'][^>]*>`)
	for _, match := range reImg.FindAllStringSubmatch(content, -1) {
		if len(match) >= 2 {
			if url := strings.TrimSpace(match[1]); url != "" {
				urls = append(urls, url)
			}
		}
	}

	// 2. Match Markdown image syntax: ![alt](url)
	reMarkdown := regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	for _, match := range reMarkdown.FindAllStringSubmatch(content, -1) {
		if len(match) >= 2 {
			if url := strings.TrimSpace(match[1]); url != "" {
				urls = append(urls, url)
			}
		}
	}

	// 3. Match reference-style Markdown images: ![alt][ref]
	reRef := regexp.MustCompile(`!\[[^\]]*\]\[([^\]]+)\]`)
	for _, match := range reRef.FindAllStringSubmatch(content, -1) {
		if len(match) >= 2 {
			ref := strings.TrimSpace(match[1])
			reDef := regexp.MustCompile(`^\s*\[` + regexp.QuoteMeta(ref) + `\]:\s*(\S+)`)
			for line := range strings.SplitSeq(content, "\n") {
				if defMatch := reDef.FindStringSubmatch(line); len(defMatch) >= 2 {
					if url := strings.TrimSpace(defMatch[1]); url != "" {
						urls = append(urls, url)
					}
					break
				}
			}
		}
	}

	return urls
}
