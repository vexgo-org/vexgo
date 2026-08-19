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

var (
	ErrPostNotFound    = errors.New("post not found")
	ErrForbidden       = errors.New("forbidden")
	ErrGuestViewDenied = errors.New("guest view denied")
	ErrBadRequest      = errors.New("bad request")
)

type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
	Notifier  Notifier
	Files     FileRemover
}

type (
	Notifier    = model.Notifier
	FileRemover = model.FileRemover
)

type Service struct {
	repo     Repository
	notifier Notifier
	files    FileRemover
}

func NewService(deps Deps) *Service {
	return &Service{repo: NewRepository(deps.DB), notifier: deps.Notifier, files: deps.Files}
}

func (s *Service) allowGuestView(ctx context.Context) bool {
	return s.repo.GetGuestViewSetting(ctx)
}

func (s *Service) List(ctx context.Context, userRole string, userID uint, page, limit int, categoryID, status, search string) ([]model.Post, int64, error) {
	if userRole == "" && !s.allowGuestView(ctx) {
		return []model.Post{}, 0, nil
	}

	query := s.repo.BaseQuery(ctx)

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

	s.populateCounts(ctx, posts, userID)

	for i := range posts {
		if userRole != model.RoleAdmin && userRole != model.RoleSuperAdmin && posts[i].AuthorID != userID {
			auth.FilterUserByPrivacy(&posts[i].Author, userID, userRole)
		}
	}

	return posts, total, nil
}

func (s *Service) Get(ctx context.Context, id, currentUserRole string, currentUserID uint) (*model.Post, error) {
	post, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("post load: %w", err)
	}

	if currentUserRole == "" && !s.allowGuestView(ctx) {
		return nil, ErrGuestViewDenied
	}

	if currentUserRole != model.RoleAdmin && currentUserRole != model.RoleSuperAdmin && post.AuthorID != currentUserID {
		auth.FilterUserByPrivacy(&post.Author, currentUserID, currentUserRole)
	}

	s.repo.IncrementViewCount(ctx, post.ID)

	count, _ := s.repo.CountLikes(ctx, post.ID)
	post.LikesCount = int(count)
	post.IsLiked = false
	if currentUserID != 0 {
		if _, err := s.repo.FindLike(ctx, post.ID, currentUserID); err == nil {
			post.IsLiked = true
		}
	}

	ccount, _ := s.repo.CountComments(ctx, post.ID)
	post.CommentsCount = int(ccount)

	return post, nil
}

type CreateRequest struct {
	Title      string
	Content    string
	Category   any
	Tags       []string
	Excerpt    string
	CoverImage string
	Status     model.PostStatus
}

func (s *Service) Create(ctx context.Context, userRole string, userID uint, req CreateRequest) (*model.Post, error) {
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

type UpdateRequest struct {
	Title      string
	Content    string
	Category   any
	Tags       []string
	Excerpt    string
	CoverImage string
	Status     model.PostStatus
}

func (s *Service) Update(ctx context.Context, id string, userID uint, req UpdateRequest) (*model.Post, error) {
	post, err := s.repo.FindByIDPreloadTags(ctx, id)
	if err != nil {
		return nil, ErrPostNotFound
	}

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

	s.repo.Save(ctx, post)
	return post, nil
}

func (s *Service) Delete(ctx context.Context, id string, userID uint) error {
	post, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return ErrPostNotFound
	}

	user, err := s.repo.FindUserByID(ctx, userID)
	if err == nil {
		if user.Role != model.RoleAdmin && user.Role != model.RoleSuperAdmin && post.AuthorID != userID {
			return ErrForbidden
		}
	}

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

	if err := s.repo.DeleteCommentsByPostID(ctx, post.ID); err != nil {
		return fmt.Errorf("delete comments: %w", err)
	}
	if err := s.repo.DeleteLikesByPostID(ctx, post.ID); err != nil {
		return fmt.Errorf("delete likes: %w", err)
	}
	if err := s.repo.ClearTagsAssociation(ctx, post); err != nil {
		return fmt.Errorf("clear tags: %w", err)
	}
	if err := s.repo.Delete(ctx, post); err != nil {
		return fmt.Errorf("delete post: %w", err)
	}

	return nil
}

func (s *Service) MyPosts(ctx context.Context, userID uint, page, limit int, status string) ([]model.Post, int64, error) {
	query := s.repo.BaseQuery(ctx).
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

	s.populateCounts(ctx, posts, userID)
	return posts, total, nil
}

func (s *Service) Drafts(ctx context.Context, userRole string, userID uint, page, limit int) ([]model.Post, int64, error) {
	query := s.repo.BaseQuery(ctx)

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

	s.populateCounts(ctx, posts, userID)
	return posts, total, nil
}

func (s *Service) UserPosts(ctx context.Context, userIDStr, currentUserRole string, currentUserID uint, page, limit int) ([]model.Post, int64, error) {
	uid, err := strconv.Atoi(userIDStr)
	if err != nil {
		return nil, 0, ErrBadRequest
	}

	if currentUserRole == "" && !s.allowGuestView(ctx) {
		return []model.Post{}, 0, nil
	}

	query := s.repo.BaseQuery(ctx).Where("author_id = ?", uid)

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

	s.populateCounts(ctx, posts, currentUserID)

	for i := range posts {
		if currentUserRole != model.RoleAdmin && currentUserRole != model.RoleSuperAdmin && uint(uid) != currentUserID {
			auth.FilterUserByPrivacy(&posts[i].Author, currentUserID, currentUserRole)
		}
	}

	return posts, total, nil
}

func (s *Service) Popular(ctx context.Context, userRole string, limit int) ([]model.Post, error) {
	if userRole == "" && !s.allowGuestView(ctx) {
		return []model.Post{}, nil
	}

	var posts []model.Post
	s.repo.BaseQuery(ctx).Where("status = ?", model.PostStatusPublished).Find(&posts)

	for i := range posts {
		count, _ := s.repo.CountLikes(ctx, posts[i].ID)
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

func (s *Service) Latest(ctx context.Context, userRole string, limit int) ([]model.Post, error) {
	if userRole == "" && !s.allowGuestView(ctx) {
		return []model.Post{}, nil
	}

	var posts []model.Post
	s.repo.BaseQuery(ctx).Where("status = ?", model.PostStatusPublished).
		Order("created_at DESC").Limit(limit).Find(&posts)

	return posts, nil
}

func (s *Service) Categories(ctx context.Context, userRole string) ([]model.Category, error) {
	if userRole == "" && !s.allowGuestView(ctx) {
		return []model.Category{}, nil
	}
	return s.repo.FindAllCategories(ctx)
}

func (s *Service) CreateCategory(ctx context.Context, name, description string) (*model.Category, error) {
	category := &model.Category{Name: name, Description: description}
	if err := s.repo.CreateCategory(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}

func (s *Service) Tags(ctx context.Context, userRole string) ([]model.Tag, error) {
	if userRole == "" && !s.allowGuestView(ctx) {
		return []model.Tag{}, nil
	}
	return s.repo.FindAllTags(ctx)
}

func (s *Service) CreateTag(ctx context.Context, name string) (*model.Tag, error) {
	return s.repo.FindOrCreateTag(ctx, name)
}

func (s *Service) ListModeration(ctx context.Context, status model.PostStatus, page, limit int, search string) ([]model.Post, int64, error) {
	offset := (page - 1) * limit
	return s.repo.ListModeration(ctx, status, offset, limit, search)
}

func (s *Service) Approve(ctx context.Context, id string) (*model.Post, error) {
	post, err := s.repo.FindByIDPreloadTags(ctx, id)
	if err != nil {
		return nil, ErrPostNotFound
	}

	post.Status = model.PostStatusPublished
	s.repo.Save(ctx, post)

	if err := s.notifier.CreateNotification(ctx,
		post.AuthorID, "review", "Post approved",
		fmt.Sprintf("Your post \"%s\" has been approved", post.Title), id, "post",
	); err != nil {
		logrus.WithError(err).Warn("failed to create post approved notification")
	}

	return post, nil
}

func (s *Service) Reject(ctx context.Context, id, rejectionReason string) (*model.Post, error) {
	post, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrPostNotFound
	}

	post.Status = model.PostStatusRejected
	post.RejectionReason = rejectionReason
	s.repo.Save(ctx, post)

	if err := s.notifier.CreateNotification(ctx,
		post.AuthorID, "review", "post rejected",
		fmt.Sprintf("Your post \"%s\" has been rejected, reason: %s", post.Title, rejectionReason), id, "post",
	); err != nil {
		logrus.WithError(err).Warn("failed to create post rejected notification")
	}

	return post, nil
}

func (s *Service) Resubmit(ctx context.Context, id string) (*model.Post, error) {
	post, err := s.repo.FindByIDPreloadTags(ctx, id)
	if err != nil {
		return nil, ErrPostNotFound
	}

	if post.Status != model.PostStatusRejected {
		return nil, ErrBadRequest
	}

	post.Status = model.PostStatusPending
	post.RejectionReason = ""
	s.repo.Save(ctx, post)

	return post, nil
}

func (s *Service) ToggleLike(ctx context.Context, postID, userID uint) (isLiked bool, count int64, err error) {
	existing, err := s.repo.FindLike(ctx, postID, userID)
	if err == nil {
		if err := s.repo.DeleteLike(ctx, existing); err != nil {
			return false, 0, err
		}
		c, _ := s.repo.CountLikes(ctx, postID)
		return false, c, nil
	}

	like := &model.Like{PostID: postID, UserID: userID}
	s.repo.CreateLike(ctx, like)
	count, _ = s.repo.CountLikes(ctx, postID)

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

func (s *Service) LikeStatus(ctx context.Context, postID, userID uint) (isLiked bool, count int64) {
	if userID != 0 {
		if _, err := s.repo.FindLike(ctx, postID, userID); err == nil {
			isLiked = true
		}
	}
	count, _ = s.repo.CountLikes(ctx, postID)
	return isLiked, count
}

func (s *Service) populateCounts(ctx context.Context, posts []model.Post, userID uint) {
	for i := range posts {
		count, _ := s.repo.CountLikes(ctx, posts[i].ID)
		posts[i].LikesCount = int(count)

		posts[i].IsLiked = false
		if userID != 0 {
			if _, err := s.repo.FindLike(ctx, posts[i].ID, userID); err == nil {
				posts[i].IsLiked = true
			}
		}

		ccount, _ := s.repo.CountComments(ctx, posts[i].ID)
		posts[i].CommentsCount = int(ccount)
	}
}

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
