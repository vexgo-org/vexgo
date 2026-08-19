// Package post implements the post, like, category, tag and post-moderation
// domain.
package post

import (
	"context"
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
	ErrPostNotFound    = errors.New("post not found")
	ErrForbidden       = errors.New("forbidden")
	ErrGuestViewDenied = errors.New("guest view denied")
	ErrBadRequest      = errors.New("bad request")
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
	repo     Repository
	notifier Notifier
	files    FileRemover
}

// NewService creates a post service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{repo: NewRepository(deps.DB), notifier: deps.Notifier, files: deps.Files}
}

func (s *Service) allowGuestView() bool {
	return s.repo.GetGuestViewSetting()
}

// List returns the paginated post list with role-based visibility, filters,
// and per-post like/comment counts.
func (s *Service) List(userRole string, userID uint, page, limit int, categoryID, status, search string) ([]model.Post, int64, error) {
	if userRole == "" && !s.allowGuestView() {
		return []model.Post{}, 0, nil
	}

	query := s.repo.BaseQuery()

	// Role-based visibility
	switch userRole {
	case "", model.RoleGuest:
		query = query.Where("status = ?", model.PostStatusPublished)
	case model.RoleContributor:
		query = query.Where(
			query.Session(&gorm.Session{}).Where("status = ?", model.PostStatusPublished).
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
	if categoryID != "" {
		if cid, err := strconv.Atoi(categoryID); err == nil {
			query = query.Where("category = ?", cid)
		} else {
			query = query.Where("category = ?", categoryID)
		}
	}
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
	post, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("post load: %w", err)
	}

	if currentUserRole == "" && !s.allowGuestView() {
		return nil, ErrGuestViewDenied
	}

	if currentUserRole != model.RoleAdmin && currentUserRole != model.RoleSuperAdmin && post.AuthorID != currentUserID {
		auth.FilterUserByPrivacy(&post.Author, currentUserID, currentUserRole)
	}

	s.repo.IncrementViewCount(post.ID)

	count, _ := s.repo.CountLikes(post.ID)
	post.LikesCount = int(count)
	post.IsLiked = false
	if currentUserID != 0 {
		if _, err := s.repo.FindLike(post.ID, currentUserID); err == nil {
			post.IsLiked = true
		}
	}

	ccount, _ := s.repo.CountComments(post.ID)
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
func (s *Service) Create(userRole string, userID uint, req CreateRequest) (*model.Post, error) {
	if userRole == "" || userRole == model.RoleGuest {
		return nil, ErrForbidden
	}

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

	if err := s.repo.Create(&post); err != nil {
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
	post, err := s.repo.FindByIDPreloadTags(id)
	if err != nil {
		return nil, ErrPostNotFound
	}

	user, err := s.repo.FindUserByID(userID)
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
		tags, err := s.resolveTags(req.Tags)
		if err == nil {
			if err := s.repo.ReplaceTagsAssociation(post, tags); err != nil {
				return nil, fmt.Errorf("replace tags: %w", err)
			}
			post.Tags = tags
		}
	}

	s.repo.Save(post)
	return post, nil
}

// Delete removes a post (author or admin only), cleaning up its files,
// comments, likes and tag associations.
func (s *Service) Delete(id string, userID uint) error {
	post, err := s.repo.FindByID(id)
	if err != nil {
		return ErrPostNotFound
	}

	user, err := s.repo.FindUserByID(userID)
	if err == nil {
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
			logrus.WithError(err).WithField("url", url).Warn("Failed to delete image")
		}
	}

	if err := s.repo.DeleteCommentsByPostID(post.ID); err != nil {
		return fmt.Errorf("delete comments: %w", err)
	}
	if err := s.repo.DeleteLikesByPostID(post.ID); err != nil {
		return fmt.Errorf("delete likes: %w", err)
	}
	if err := s.repo.ClearTagsAssociation(post); err != nil {
		return fmt.Errorf("clear tags: %w", err)
	}
	if err := s.repo.Delete(post); err != nil {
		return fmt.Errorf("delete post: %w", err)
	}

	return nil
}

// MyPosts returns the current user's own posts (excluding rejected).
func (s *Service) MyPosts(userID uint, page, limit int, status string) ([]model.Post, int64, error) {
	query := s.repo.BaseQuery().
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
	query := s.repo.BaseQuery()

	if userRole != "" && (userRole == model.RoleAdmin || userRole == model.RoleSuperAdmin) {
		query = query.Where("status = ?", model.PostStatusDraft)
	} else {
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
	uid, err := strconv.Atoi(userIDStr)
	if err != nil {
		return nil, 0, ErrBadRequest
	}

	if currentUserRole == "" && !s.allowGuestView() {
		return []model.Post{}, 0, nil
	}

	query := s.repo.BaseQuery().Where("author_id = ?", uid)

	switch currentUserRole {
	case "", model.RoleGuest:
		query = query.Where("status = ?", model.PostStatusPublished)
	case model.RoleContributor:
		if uint(uid) != currentUserID {
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
		if currentUserRole != model.RoleAdmin && currentUserRole != model.RoleSuperAdmin && uint(uid) != currentUserID {
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
	s.repo.BaseQuery().Where("status = ?", model.PostStatusPublished).Find(&posts)

	for i := range posts {
		count, _ := s.repo.CountLikes(posts[i].ID)
		posts[i].LikesCount = int(count)
	}

	for i := 0; i < len(posts); i++ {
		for j := i + 1; j < len(posts); j++ {
			scoreI := posts[i].LikesCount*5 + posts[i].ViewCount
			scoreJ := posts[j].LikesCount*5 + posts[j].ViewCount
			if scoreJ > scoreI {
				posts[i], posts[j] = posts[j], posts[i]
			}
		}
	}

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
	s.repo.BaseQuery().Where("status = ?", model.PostStatusPublished).
		Order("created_at DESC").Limit(limit).Find(&posts)

	return posts, nil
}

// Categories returns all categories.
func (s *Service) Categories(userRole string) ([]model.Category, error) {
	if userRole == "" && !s.allowGuestView() {
		return []model.Category{}, nil
	}
	return s.repo.FindAllCategories()
}

// CreateCategory creates a category.
func (s *Service) CreateCategory(name, description string) (*model.Category, error) {
	category := &model.Category{Name: name, Description: description}
	if err := s.repo.CreateCategory(category); err != nil {
		return nil, err
	}
	return category, nil
}

// Tags returns all tags.
func (s *Service) Tags(userRole string) ([]model.Tag, error) {
	if userRole == "" && !s.allowGuestView() {
		return []model.Tag{}, nil
	}
	return s.repo.FindAllTags()
}

// CreateTag creates a tag.
func (s *Service) CreateTag(name string) (*model.Tag, error) {
	return s.repo.FindOrCreateTag(name)
}

// ListModeration returns the paginated posts with the given status for the
// moderation queue, with an optional search across title/content/username.
func (s *Service) ListModeration(status model.PostStatus, page, limit int, search string) ([]model.Post, int64, error) {
	offset := (page - 1) * limit
	return s.repo.ListModeration(status, offset, limit, search)
}

// Approve approves a post and notifies its author.
func (s *Service) Approve(id string) (*model.Post, error) {
	post, err := s.repo.FindByIDPreloadTags(id)
	if err != nil {
		return nil, ErrPostNotFound
	}

	post.Status = model.PostStatusPublished
	s.repo.Save(post)

	if err := s.notifier.CreateNotification(context.Background(),
		post.AuthorID,
		"review",
		"Post approved",
		fmt.Sprintf("Your post \"%s\" has been approved", post.Title),
		id,
		"post",
	); err != nil {
		logrus.WithError(err).Warn("failed to create post approved notification")
	}

	return post, nil
}

// Reject rejects a post with a reason and notifies its author.
func (s *Service) Reject(id, rejectionReason string) (*model.Post, error) {
	post, err := s.repo.FindByID(id)
	if err != nil {
		return nil, ErrPostNotFound
	}

	post.Status = model.PostStatusRejected
	post.RejectionReason = rejectionReason
	s.repo.Save(post)

	if err := s.notifier.CreateNotification(context.Background(),
		post.AuthorID,
		"review",
		"post rejected",
		fmt.Sprintf("Your post \"%s\" has been rejected, reason: %s", post.Title, rejectionReason),
		id,
		"post",
	); err != nil {
		logrus.WithError(err).Warn("failed to create post rejected notification")
	}

	return post, nil
}

// Resubmit moves a rejected post back to pending.
func (s *Service) Resubmit(id string) (*model.Post, error) {
	post, err := s.repo.FindByIDPreloadTags(id)
	if err != nil {
		return nil, ErrPostNotFound
	}

	if post.Status != model.PostStatusRejected {
		return nil, ErrBadRequest
	}

	post.Status = model.PostStatusPending
	post.RejectionReason = ""
	s.repo.Save(post)

	return post, nil
}

// ToggleLike likes or unlikes a post and notifies the author on like.
func (s *Service) ToggleLike(postID, userID uint) (isLiked bool, count int64, err error) {
	existing, err := s.repo.FindLike(postID, userID)
	if err == nil {
		// Already liked -> unlike
		if err := s.repo.DeleteLike(existing); err != nil {
			return false, 0, err
		}
		c, _ := s.repo.CountLikes(postID)
		return false, c, nil
	}

	like := &model.Like{PostID: postID, UserID: userID}
	s.repo.CreateLike(like)
	count, _ = s.repo.CountLikes(postID)

	// Create notification for post author
	post, err := s.repo.FindByID(fmt.Sprintf("%d", postID))
	if err == nil && post.AuthorID != userID {
		user, err := s.repo.FindUserByID(userID)
		if err == nil {
			if err := s.notifier.CreateNotification(context.Background(),
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

	return true, count, nil
}

// LikeStatus returns whether the user liked the post and the total like count.
func (s *Service) LikeStatus(postID, userID uint) (isLiked bool, count int64) {
	if userID != 0 {
		if _, err := s.repo.FindLike(postID, userID); err == nil {
			isLiked = true
		}
	}
	count, _ = s.repo.CountLikes(postID)
	return isLiked, count
}

// populateCounts fills like/comment counts and the current user's like status.
func (s *Service) populateCounts(posts []model.Post, userID uint) {
	for i := range posts {
		count, _ := s.repo.CountLikes(posts[i].ID)
		posts[i].LikesCount = int(count)

		posts[i].IsLiked = false
		if userID != 0 {
			if _, err := s.repo.FindLike(posts[i].ID, userID); err == nil {
				posts[i].IsLiked = true
			}
		}

		ccount, _ := s.repo.CountComments(posts[i].ID)
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
		tag, err := s.repo.FindOrCreateTag(name)
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

	reImg := regexp.MustCompile(`<img[^>]+src=["']([^"']+)["'][^>]*>`)
	for _, match := range reImg.FindAllStringSubmatch(content, -1) {
		if len(match) >= 2 {
			if url := strings.TrimSpace(match[1]); url != "" {
				urls = append(urls, url)
			}
		}
	}

	reMarkdown := regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	for _, match := range reMarkdown.FindAllStringSubmatch(content, -1) {
		if len(match) >= 2 {
			if url := strings.TrimSpace(match[1]); url != "" {
				urls = append(urls, url)
			}
		}
	}

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
