// Package post implements the post, like, category, tag and post-moderation
// domain.
package post

import (
	"errors"
	"fmt"
	"regexp"
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

// Notifier is an alias for model.Notifier kept for backward compatibility.
type Notifier = model.Notifier

// FileRemover is an alias for model.FileRemover kept for backward compatibility.
type FileRemover = model.FileRemover

// Service contains the business logic of the post domain.
type Service struct {
	db       *gorm.DB
	notifier Notifier
	files    FileRemover
}

// NewService creates a post service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{db: deps.DB, notifier: deps.Notifier, files: deps.Files}
}

// allowGuestView reports whether anonymous viewers may see posts.
func (s *Service) allowGuestView() bool {
	var config model.GeneralSettings
	if err := s.db.First(&config).Error; err != nil {
		return true // Default to true if config not found
	}
	return config.AllowGuestViewPosts
}

// List returns the paginated post list with role-based visibility, filters,
// and per-post like/comment counts.
func (s *Service) List(userRole string, userID uint, page, limit int, categoryID, status, search string) ([]model.Post, int64, error) {
	// If not logged in and guest viewing is not allowed, return empty result
	if userRole == "" && !s.allowGuestView() {
		return []model.Post{}, 0, nil
	}

	query := s.db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags")

	// Determine visible posts based on user role
	switch userRole {
	case "", model.RoleGuest:
		query = query.Where("status = ?", model.PostStatusPublished)
	case model.RoleContributor:
		query = query.Where(
			s.db.Where("status = ?", model.PostStatusPublished).
				Or("author_id = ? AND status != ?", userID, model.PostStatusRejected),
		)
	case model.RoleAuthor, model.RoleAdmin, model.RoleSuperAdmin:
		query = query.Where("status != ?", model.PostStatusRejected)
	default:
		query = query.Where("status = ?", model.PostStatusPublished)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	// Category filter (by id or name)
	if categoryID != "" {
		if cid, err := strconv.Atoi(categoryID); err == nil {
			query = query.Where("category = ?", cid)
		} else {
			query = query.Where("category = ?", categoryID)
		}
	}

	// Search filter
	if search != "" {
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var posts []model.Post
	if err := query.Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&posts).Error; err != nil {
		return nil, 0, err
	}

	s.populateCounts(posts, userID)

	for i := range posts {
		if userRole != model.RoleAdmin && userRole != model.RoleSuperAdmin && posts[i].AuthorID != userID {
			auth.FilterUserByPrivacy(&posts[i].Author, userID, userRole)
		}
	}

	return posts, total, nil
}

// Get returns a single post with privacy filtering, view-count increment and
// like/comment counts.
func (s *Service) Get(id, currentUserRole string, currentUserID uint) (*model.Post, error) {
	var post model.Post
	if err := s.db.Preload("Author").Preload("Tags").First(&post, id).Error; err != nil {
		return nil, fmt.Errorf("post load: %w", err)
	}

	// If not logged in and guest viewing is not allowed, return 403
	if currentUserRole == "" && !s.allowGuestView() {
		return nil, ErrGuestViewDenied
	}

	if currentUserRole != model.RoleAdmin && currentUserRole != model.RoleSuperAdmin && post.AuthorID != currentUserID {
		auth.FilterUserByPrivacy(&post.Author, currentUserID, currentUserRole)
	}

	// Increment view count (optional)
	s.db.Model(&post).UpdateColumn("view_count", gorm.Expr("view_count + ?", 1))

	// Fill likes count and current logged-in user's like status
	var count int64
	s.db.Model(&model.Like{}).Where("post_id = ?", post.ID).Count(&count)
	post.LikesCount = int(count)
	post.IsLiked = false
	if currentUserID != 0 {
		var like model.Like
		if s.db.Where("post_id = ? AND user_id = ?", post.ID, currentUserID).First(&like).Error == nil {
			post.IsLiked = true
		}
	}

	// Fill comments count
	var ccount int64
	s.db.Model(&model.Comment{}).Where("post_id = ?", post.ID).Count(&ccount)
	post.CommentsCount = int(ccount)

	return &post, nil
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
func (s *Service) Create(userRole string, userID uint, req CreateRequest) (*model.Post, error) {
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
		tags, err := s.resolveTags(req.Tags)
		if err == nil {
			post.Tags = tags
		}
	}

	if err := s.db.Create(&post).Error; err != nil {
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
func (s *Service) Update(id string, userID uint, req UpdateRequest) (*model.Post, error) {
	var post model.Post
	if err := s.db.Preload("Tags").First(&post, id).Error; err != nil {
		return nil, ErrPostNotFound
	}

	// Permission check
	var user model.User
	if err := s.db.First(&user, userID).Error; err == nil {
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
		tags, err := s.resolveTags(req.Tags)
		if err == nil {
			if err := s.db.Model(&post).Association("Tags").Replace(tags); err != nil {
				return nil, fmt.Errorf("replace tags: %w", err)
			}
			post.Tags = tags
		}
	}

	s.db.Save(&post)
	return &post, nil
}

// Delete removes a post (author or admin only), cleaning up its files,
// comments, likes and tag associations.
func (s *Service) Delete(id string, userID uint) error {
	var post model.Post
	if err := s.db.First(&post, id).Error; err != nil {
		return ErrPostNotFound
	}

	var user model.User
	if err := s.db.First(&user, userID).Error; err == nil {
		if user.Role != model.RoleAdmin && user.Role != model.RoleSuperAdmin && post.AuthorID != userID {
			return ErrForbidden
		}
	}

	// Collect image URLs to delete
	var imagesToDelete []string
	if post.CoverImage != "" {
		imagesToDelete = append(imagesToDelete, post.CoverImage)
	}
	contentImages := extractImageURLs(post.Content)
	imagesToDelete = append(imagesToDelete, contentImages...)

	uniqueImages := make(map[string]bool)
	for _, url := range imagesToDelete {
		uniqueImages[url] = true
	}
	for url := range uniqueImages {
		if err := s.files.Delete(url); err != nil {
			// Log error but continue execution to avoid post deletion failure
			logrus.WithError(err).WithField("url", url).Warn("Failed to delete image")
		}
	}

	// Delete comments associated with this post
	if err := s.db.Where("post_id = ?", post.ID).Delete(&model.Comment{}).Error; err != nil {
		return fmt.Errorf("delete comments: %w", err)
	}

	// Delete likes associated with this post
	if err := s.db.Where("post_id = ?", post.ID).Delete(&model.Like{}).Error; err != nil {
		return fmt.Errorf("delete likes: %w", err)
	}

	// Clear the many-to-many tags relationship
	if err := s.db.Model(&post).Association("Tags").Clear(); err != nil {
		return fmt.Errorf("clear tags: %w", err)
	}

	// Finally delete the post itself
	if err := s.db.Delete(&post).Error; err != nil {
		return fmt.Errorf("delete post: %w", err)
	}

	return nil
}

// MyPosts returns the current user's own posts (excluding rejected).
func (s *Service) MyPosts(userID uint, page, limit int, status string) ([]model.Post, int64, error) {
	query := s.db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags").
		Where("author_id = ? AND status != ?", userID, model.PostStatusRejected)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var posts []model.Post
	if err := query.Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&posts).Error; err != nil {
		return nil, 0, err
	}

	s.populateCounts(posts, userID)

	return posts, total, nil
}

// Drafts returns draft posts; admins see all drafts, other users only their own.
func (s *Service) Drafts(userRole string, userID uint, page, limit int) ([]model.Post, int64, error) {
	query := s.db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags")

	if userRole != "" && (userRole == model.RoleAdmin || userRole == model.RoleSuperAdmin) {
		// Admins and super admins can see all draft posts
		query = query.Where("status = ?", model.PostStatusDraft)
	} else {
		// Other users can only see their own draft posts
		query = query.Where("author_id = ? AND status = ?", userID, model.PostStatusDraft)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var posts []model.Post
	if err := query.Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&posts).Error; err != nil {
		return nil, 0, err
	}

	s.populateCounts(posts, userID)

	return posts, total, nil
}

// UserPosts returns the posts of a specific user with role-based visibility.
func (s *Service) UserPosts(userIDStr, currentUserRole string, currentUserID uint, page, limit int) ([]model.Post, int64, error) {
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		return nil, 0, ErrBadRequest
	}

	if currentUserRole == "" && !s.allowGuestView() {
		return []model.Post{}, 0, nil
	}

	query := s.db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags").
		Where("author_id = ?", userID)

	// Determine visible posts based on user role
	switch currentUserRole {
	case "", model.RoleGuest:
		query = query.Where("status = ?", model.PostStatusPublished)
	case model.RoleContributor:
		if uint(userID) != currentUserID {
			query = query.Where("status = ?", model.PostStatusPublished)
		} else {
			query = query.Where("status != ?", model.PostStatusRejected)
		}
	default:
		query = query.Where("status != ?", model.PostStatusRejected)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var posts []model.Post
	if err := query.Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&posts).Error; err != nil {
		return nil, 0, err
	}

	s.populateCounts(posts, currentUserID)

	for i := range posts {
		if currentUserRole != model.RoleAdmin && currentUserRole != model.RoleSuperAdmin && uint(userID) != currentUserID {
			auth.FilterUserByPrivacy(&posts[i].Author, currentUserID, currentUserRole)
		}
	}

	return posts, total, nil
}

// Popular returns the top posts by likes*5 + views, limited to published posts.
func (s *Service) Popular(userRole string, limit int) ([]model.Post, error) {
	if userRole == "" && !s.allowGuestView() {
		return []model.Post{}, nil
	}

	var posts []model.Post
	s.db.Where("status = ?", model.PostStatusPublished).
		Preload("Author").
		Preload("Tags").
		Find(&posts)

	// Calculate likes count for each post
	for i := range posts {
		var count int64
		s.db.Model(&model.Like{}).Where("post_id = ?", posts[i].ID).Count(&count)
		posts[i].LikesCount = int(count)
	}

	// Sort by the sum of likes count multiplied by 5 plus view count
	for i := 0; i < len(posts); i++ {
		for j := i + 1; j < len(posts); j++ {
			scoreI := posts[i].LikesCount*5 + posts[i].ViewCount
			scoreJ := posts[j].LikesCount*5 + posts[j].ViewCount
			if scoreJ > scoreI {
				posts[i], posts[j] = posts[j], posts[i]
			}
		}
	}

	// Limit the number of returned items
	if limit < len(posts) {
		posts = posts[:limit]
	}

	return posts, nil
}

// Latest returns the most recent published posts.
func (s *Service) Latest(userRole string, limit int) ([]model.Post, error) {
	if userRole == "" && !s.allowGuestView() {
		return []model.Post{}, nil
	}

	var posts []model.Post
	s.db.Where("status = ?", model.PostStatusPublished).
		Order("created_at DESC").
		Limit(limit).
		Preload("Author").
		Preload("Tags").
		Find(&posts)

	return posts, nil
}

// Categories returns all categories.
func (s *Service) Categories(userRole string) ([]model.Category, error) {
	if userRole == "" && !s.allowGuestView() {
		return []model.Category{}, nil
	}

	var categories []model.Category
	if err := s.db.Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

// CreateCategory creates a category.
func (s *Service) CreateCategory(name, description string) (*model.Category, error) {
	category := model.Category{
		Name:        name,
		Description: description,
	}
	if err := s.db.Create(&category).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

// Tags returns all tags.
func (s *Service) Tags(userRole string) ([]model.Tag, error) {
	if userRole == "" && !s.allowGuestView() {
		return []model.Tag{}, nil
	}

	var tags []model.Tag
	if err := s.db.Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

// CreateTag creates a tag.
func (s *Service) CreateTag(name string) (*model.Tag, error) {
	tag := model.Tag{Name: name}
	if err := s.db.Create(&tag).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

// ListModeration returns the paginated posts with the given status for the
// moderation queue, with an optional search across title/content/username.
func (s *Service) ListModeration(status model.PostStatus, page, limit int, search string) ([]model.Post, int64, error) {
	query := s.db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags").
		Where("status = ?", status)

	if search != "" {
		searchTerm := "%" + search + "%"
		query = query.Joins("LEFT JOIN users ON posts.author_id = users.id").
			Where("posts.title LIKE ? OR posts.content LIKE ? OR users.username LIKE ?", searchTerm, searchTerm, searchTerm)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var posts []model.Post
	if err := query.Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&posts).Error; err != nil {
		return nil, 0, err
	}

	return posts, total, nil
}

// Approve approves a post and notifies its author.
func (s *Service) Approve(id string) (*model.Post, error) {
	var post model.Post
	if err := s.db.Preload("Tags").First(&post, id).Error; err != nil {
		return nil, ErrPostNotFound
	}

	post.Status = model.PostStatusPublished
	s.db.Save(&post)

	if err := s.notifier.CreateNotification(
		post.AuthorID,
		"review",
		"Post approved",
		fmt.Sprintf("Your post \"%s\" has been approved", post.Title),
		id,
		"post",
	); err != nil {
		logrus.WithError(err).Warn("failed to create post approved notification")
	}

	return &post, nil
}

// Reject rejects a post with a reason and notifies its author.
func (s *Service) Reject(id, rejectionReason string) (*model.Post, error) {
	var post model.Post
	if err := s.db.First(&post, id).Error; err != nil {
		return nil, ErrPostNotFound
	}

	post.Status = model.PostStatusRejected
	post.RejectionReason = rejectionReason
	s.db.Save(&post)

	if err := s.notifier.CreateNotification(
		post.AuthorID,
		"review",
		"post rejected",
		fmt.Sprintf("Your post \"%s\" has been rejected, reason: %s", post.Title, rejectionReason),
		id,
		"post",
	); err != nil {
		logrus.WithError(err).Warn("failed to create post rejected notification")
	}

	return &post, nil
}

// Resubmit moves a rejected post back to pending.
func (s *Service) Resubmit(id string) (*model.Post, error) {
	var post model.Post
	if err := s.db.Preload("Tags").First(&post, id).Error; err != nil {
		return nil, ErrPostNotFound
	}

	// Check if post status is rejected
	if post.Status != model.PostStatusRejected {
		return nil, ErrBadRequest
	}

	post.Status = model.PostStatusPending
	post.RejectionReason = ""
	s.db.Save(&post)

	return &post, nil
}

// ToggleLike likes or unlikes a post and notifies the author on like.
func (s *Service) ToggleLike(postID, userID uint) (isLiked bool, count int64, err error) {
	var like model.Like
	if err := s.db.Where("post_id = ? AND user_id = ?", postID, userID).First(&like).Error; err == nil {
		// Already liked -> unlike
		if err := s.db.Delete(&like).Error; err != nil {
			return false, 0, err
		}
		var c int64
		s.db.Model(&model.Like{}).Where("post_id = ?", postID).Count(&c)
		return false, c, nil
	}

	like = model.Like{PostID: postID, UserID: userID}
	s.db.Create(&like)
	s.db.Model(&model.Like{}).Where("post_id = ?", postID).Count(&count)

	// Create notification for post author
	var post model.Post
	if err := s.db.First(&post, postID).Error; err == nil {
		if post.AuthorID != userID { // Don't notify the user if they are the post author
			var user model.User
			if err := s.db.First(&user, userID).Error; err == nil {
				if err := s.notifier.CreateNotification(
					post.AuthorID,
					"like",
					"The post received likes",
					fmt.Sprintf("User \"%s\" liked your post \"%s\"", user.Username, post.Title),
					strconv.FormatUint(uint64(postID), 10),
					"post",
				); err != nil {
					logrus.WithError(err).Warn("failed to create like notification")
				}
			}
		}
	}

	return true, count, nil
}

// LikeStatus returns whether the user liked the post and the total like count.
func (s *Service) LikeStatus(postID, userID uint) (isLiked bool, count int64) {
	if userID != 0 {
		var like model.Like
		if s.db.Where("post_id = ? AND user_id = ?", postID, userID).First(&like).Error == nil {
			isLiked = true
		}
	}
	s.db.Model(&model.Like{}).Where("post_id = ?", postID).Count(&count)
	return isLiked, count
}

// populateCounts fills like/comment counts and the current user's like status.
func (s *Service) populateCounts(posts []model.Post, userID uint) {
	for i := range posts {
		var count int64
		s.db.Model(&model.Like{}).Where("post_id = ?", posts[i].ID).Count(&count)
		posts[i].LikesCount = int(count)

		posts[i].IsLiked = false
		if userID != 0 {
			var like model.Like
			if s.db.Where("post_id = ? AND user_id = ?", posts[i].ID, userID).First(&like).Error == nil {
				posts[i].IsLiked = true
			}
		}

		var ccount int64
		s.db.Model(&model.Comment{}).Where("post_id = ?", posts[i].ID).Count(&ccount)
		posts[i].CommentsCount = int(ccount)
	}
}

// resolveTags takes names and returns Tag models (creating missing ones).
func (s *Service) resolveTags(names []string) ([]model.Tag, error) {
	var tags []model.Tag
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		var tag model.Tag
		if err := s.db.Where("name = ?", name).First(&tag).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				tag = model.Tag{Name: name}
				s.db.Create(&tag)
			} else {
				return nil, err
			}
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

// extractImageURLs returns all image URLs referenced in HTML or Markdown content.
func extractImageURLs(content string) []string {
	var urls []string

	// 1. Match src attribute of <img> tags (HTML)
	reImg := regexp.MustCompile(`<img[^>]+src=["']([^"']+)["'][^>]*>`)
	matches := reImg.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			url := strings.TrimSpace(match[1])
			if url != "" {
				urls = append(urls, url)
			}
		}
	}

	// 2. Match Markdown image syntax: ![alt](url)
	reMarkdown := regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	matches = reMarkdown.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			url := strings.TrimSpace(match[1])
			if url != "" {
				urls = append(urls, url)
			}
		}
	}

	// 3. Match reference-style Markdown images: ![alt][ref]
	reRef := regexp.MustCompile(`!\[[^\]]*\]\[([^]]+)\]`)
	matches = reRef.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			ref := strings.TrimSpace(match[1])
			reDef := regexp.MustCompile(`^\s*\[` + regexp.QuoteMeta(ref) + `\]:\s*(\S+)`)
			for line := range strings.SplitSeq(content, "\n") {
				if defMatch := reDef.FindStringSubmatch(line); len(defMatch) >= 2 {
					url := strings.TrimSpace(defMatch[1])
					if url != "" {
						urls = append(urls, url)
					}
					break
				}
			}
		}
	}

	return urls
}
