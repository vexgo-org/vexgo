// Package post implements the post, like, category, tag and post-moderation
// domain.
package post

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"vexgo/backend/internal/auth"
	"vexgo/backend/internal/model"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
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
)

// Deps holds the dependencies required by the post domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
	Notifier  Notifier
	Files     FileRemover
}

// Notifier is the seam for creating notifications; implemented by the message domain.
// FileRemover deletes a stored file by its public URL; implemented by upload.Storage.
type (
	Notifier    = model.Notifier
	FileRemover = model.FileRemover
)

// Service contains the business logic of the post domain.
type Service struct {
	repo     Repository
	notifier Notifier
	files    FileRemover
}

// NewService creates a post service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{repo: NewRepository(deps.DB), notifier: deps.Notifier, files: deps.Files}
}

// allowGuestView reports whether anonymous viewers may see posts.
func (s *Service) allowGuestView(ctx context.Context) bool {
	return s.repo.GetGuestViewSetting(ctx)
}

// List returns the paginated post list with role-based visibility, filters,
// and per-post like/comment counts.
func (s *Service) List(ctx context.Context, userRole string, userID uint, page, limit int, categoryID, status, search string) ([]model.Post, int64, error) {
	// If not logged in and guest viewing is not allowed, return empty result
	if userRole == "" && !s.allowGuestView(ctx) {
		return []model.Post{}, 0, nil
	}

	posts, total, err := s.repo.List(ctx, userRole, userID, ListFilter{
		Page: page, Limit: limit, CategoryID: categoryID, Search: search,
	})
	if err != nil {
		return nil, 0, err
	}

	s.populateCounts(ctx, posts, userID)

	// Apply privacy filtering to author information
	for i := range posts {
		if userRole != model.RoleAdmin && userRole != model.RoleSuperAdmin && posts[i].AuthorID != userID {
			auth.FilterUserByPrivacy(&posts[i].Author, userID, userRole)
		}
	}

	return posts, total, nil
}

// Get returns a single post with privacy filtering, view-count increment and
// like/comment counts.
func (s *Service) Get(ctx context.Context, id, currentUserRole string, currentUserID uint) (*model.Post, error) {
	post, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("post load: %w", err)
	}

	// If not logged in and guest viewing is not allowed, return 403
	if currentUserRole == "" && !s.allowGuestView(ctx) {
		return nil, ErrGuestViewDenied
	}

	if currentUserRole != model.RoleAdmin && currentUserRole != model.RoleSuperAdmin && post.AuthorID != currentUserID {
		auth.FilterUserByPrivacy(&post.Author, currentUserID, currentUserRole)
	}

	// Increment view count (best-effort)
	if err := s.repo.IncrementViewCount(ctx, post.ID); err != nil {
		logrus.WithError(err).Warn("failed to increment view count")
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
		if err == nil {
			post.Tags = tags
		}
	}

	if err := s.repo.Create(ctx, &post); err != nil {
		return nil, err
	}

	return &post, nil
}

// UpdateRequest carries the fields accepted when updating a post.
type UpdateRequest struct {
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
	if err == nil {
		if user.Role != model.RoleAdmin && user.Role != model.RoleSuperAdmin && post.AuthorID != userID {
			return nil, ErrForbidden
		}
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
		if err == nil {
			if err := s.repo.ReplaceTagsAssociation(ctx, post, tags); err != nil {
				return nil, fmt.Errorf("replace tags: %w", err)
			}
			post.Tags = tags
		}
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
	if err == nil {
		if user.Role != model.RoleAdmin && user.Role != model.RoleSuperAdmin && post.AuthorID != userID {
			return ErrForbidden
		}
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
			logrus.WithError(err).WithField("url", url).Warn("Failed to delete image")
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

// MyPosts returns the current user's own posts (excluding rejected).
func (s *Service) MyPosts(ctx context.Context, userID uint, page, limit int, status string) ([]model.Post, int64, error) {
	posts, total, err := s.repo.MyPosts(ctx, userID, ListFilter{Page: page, Limit: limit, Status: status})
	if err != nil {
		return nil, 0, err
	}

	s.populateCounts(ctx, posts, userID)
	return posts, total, nil
}

// Drafts returns draft posts; admins see all drafts, other users only their own.
func (s *Service) Drafts(ctx context.Context, userRole string, userID uint, page, limit int) ([]model.Post, int64, error) {
	posts, total, err := s.repo.Drafts(ctx, userRole, userID, ListFilter{Page: page, Limit: limit})
	if err != nil {
		return nil, 0, err
	}

	s.populateCounts(ctx, posts, userID)
	return posts, total, nil
}

// UserPosts returns the posts of a specific user with role-based visibility.
func (s *Service) UserPosts(ctx context.Context, userIDStr, currentUserRole string, currentUserID uint, page, limit int) ([]model.Post, int64, error) {
	uid, err := strconv.Atoi(userIDStr)
	if err != nil {
		return nil, 0, ErrBadRequest
	}

	if currentUserRole == "" && !s.allowGuestView(ctx) {
		return []model.Post{}, 0, nil
	}

	posts, total, err := s.repo.UserPosts(ctx, uint(uid), currentUserRole, currentUserID, ListFilter{Page: page, Limit: limit})
	if err != nil {
		return nil, 0, err
	}

	s.populateCounts(ctx, posts, currentUserID)

	// Apply privacy filtering to author information
	for i := range posts {
		if currentUserRole != model.RoleAdmin && currentUserRole != model.RoleSuperAdmin && uint(uid) != currentUserID {
			auth.FilterUserByPrivacy(&posts[i].Author, currentUserID, currentUserRole)
		}
	}

	return posts, total, nil
}

// Popular returns the top posts by likes*5 + views, limited to published posts.
func (s *Service) Popular(ctx context.Context, userRole string, limit int) ([]model.Post, error) {
	if userRole == "" && !s.allowGuestView(ctx) {
		return []model.Post{}, nil
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

// Categories returns all categories.
func (s *Service) Categories(ctx context.Context, userRole string) ([]model.Category, error) {
	if userRole == "" && !s.allowGuestView(ctx) {
		return []model.Category{}, nil
	}
	return s.repo.FindAllCategories(ctx)
}

// CreateCategory creates a category.
func (s *Service) CreateCategory(ctx context.Context, name, description string) (*model.Category, error) {
	category := &model.Category{Name: name, Description: description}
	if err := s.repo.CreateCategory(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}

// Tags returns all tags.
func (s *Service) Tags(ctx context.Context, userRole string) ([]model.Tag, error) {
	if userRole == "" && !s.allowGuestView(ctx) {
		return []model.Tag{}, nil
	}
	return s.repo.FindAllTags(ctx)
}

// CreateTag creates a tag.
func (s *Service) CreateTag(ctx context.Context, name string) (*model.Tag, error) {
	return s.repo.FindOrCreateTag(ctx, name)
}

// ListModeration returns the paginated posts with the given status for the
// moderation queue, with an optional search across title/content/username.
func (s *Service) ListModeration(ctx context.Context, status model.PostStatus, page, limit int, search string) ([]model.Post, int64, error) {
	offset := (page - 1) * limit
	return s.repo.ListModeration(ctx, status, offset, limit, search)
}

// Approve approves a post and notifies its author.
func (s *Service) Approve(ctx context.Context, id string) (*model.Post, error) {
	post, err := s.repo.FindByIDPreloadTags(ctx, id)
	if err != nil {
		return nil, ErrPostNotFound
	}

	post.Status = model.PostStatusPublished
	if err := s.repo.Save(ctx, post); err != nil {
		return nil, fmt.Errorf("save post: %w", err)
	}

	if err := s.notifier.CreateNotification(ctx,
		post.AuthorID, "review", "Post approved",
		fmt.Sprintf("Your post \"%s\" has been approved", post.Title), id, "post",
	); err != nil {
		logrus.WithError(err).Warn("failed to create post approved notification")
	}

	return post, nil
}

// Reject rejects a post with a reason and notifies its author.
func (s *Service) Reject(ctx context.Context, id, rejectionReason string) (*model.Post, error) {
	post, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrPostNotFound
	}

	post.Status = model.PostStatusRejected
	post.RejectionReason = rejectionReason
	if err := s.repo.Save(ctx, post); err != nil {
		return nil, fmt.Errorf("save post: %w", err)
	}

	if err := s.notifier.CreateNotification(ctx,
		post.AuthorID, "review", "post rejected",
		fmt.Sprintf("Your post \"%s\" has been rejected, reason: %s", post.Title, rejectionReason), id, "post",
	); err != nil {
		logrus.WithError(err).Warn("failed to create post rejected notification")
	}

	return post, nil
}

// Resubmit moves a rejected post back to pending.
func (s *Service) Resubmit(ctx context.Context, id string) (*model.Post, error) {
	post, err := s.repo.FindByIDPreloadTags(ctx, id)
	if err != nil {
		return nil, ErrPostNotFound
	}

	// Check if post status is rejected
	if post.Status != model.PostStatusRejected {
		return nil, ErrBadRequest
	}

	post.Status = model.PostStatusPending
	post.RejectionReason = ""
	if err := s.repo.Save(ctx, post); err != nil {
		return nil, fmt.Errorf("save post: %w", err)
	}

	return post, nil
}

// ToggleLike likes or unlikes a post and notifies the author on like.
func (s *Service) ToggleLike(ctx context.Context, postID, userID uint) (isLiked bool, count int64, err error) {
	existing, err := s.repo.FindLike(ctx, postID, userID)
	if err == nil {
		// Already liked -> unlike
		if err := s.repo.DeleteLike(ctx, existing); err != nil {
			return false, 0, err
		}
		c, _ := s.repo.CountLikes(ctx, postID)
		return false, c, nil
	}

	like := &model.Like{PostID: postID, UserID: userID}
	if err := s.repo.CreateLike(ctx, like); err != nil {
		return false, 0, err
	}
	count, _ = s.repo.CountLikes(ctx, postID)

	// Create notification for post author
	post, err := s.repo.FindByID(ctx, fmt.Sprintf("%d", postID))
	if err == nil && post.AuthorID != userID {
		user, err := s.repo.FindUserByID(ctx, userID)
		if err == nil {
			if err := s.notifier.CreateNotification(ctx,
				post.AuthorID, "like", "The post received likes",
				fmt.Sprintf("User \"%s\" liked your post \"%s\"", user.Username, post.Title),
				strconv.FormatUint(uint64(postID), 10), "post",
			); err != nil {
				logrus.WithError(err).Warn("failed to create like notification")
			}
		}
	}

	return true, count, nil
}

// LikeStatus returns whether the user liked the post and the total like count.
func (s *Service) LikeStatus(ctx context.Context, postID, userID uint) (isLiked bool, count int64) {
	if userID != 0 {
		if _, err := s.repo.FindLike(ctx, postID, userID); err == nil {
			isLiked = true
		}
	}
	count, _ = s.repo.CountLikes(ctx, postID)
	return isLiked, count
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

// resolveTags takes names and returns Tag models (creating missing ones).
func (s *Service) resolveTags(ctx context.Context, names []string) ([]model.Tag, error) {
	var tags []model.Tag
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		tag, err := s.repo.FindOrCreateTag(ctx, name)
		if err != nil {
			return nil, err
		}
		tags = append(tags, *tag)
	}
	return tags, nil
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
