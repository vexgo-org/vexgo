package post

import (
	"context"
	"errors"
	"strconv"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ListFilter groups the list-query parameters (pagination, status and search
// filtering). Each query method uses only the fields it needs.
type ListFilter struct {
	Page, Limit int
	CategoryID  string
	Status      string
	Search      string
}

// Repository is the persistence interface for the post domain.
type Repository interface {
	// Posts
	FindByID(ctx context.Context, id string) (*model.Post, error)
	FindByIDPreloadTags(ctx context.Context, id string) (*model.Post, error)
	FindBySlug(ctx context.Context, slug string) (*model.Post, error)
	FindBySlugExcludeID(ctx context.Context, slug string, excludeID uint) (*model.Post, error)
	SlugExists(ctx context.Context, slug string) (bool, error)
	SlugExistsExcludeID(ctx context.Context, slug string, excludeID uint) (bool, error)
	Create(ctx context.Context, post *model.Post) error
	Save(ctx context.Context, post *model.Post) error
	Delete(ctx context.Context, post *model.Post) error
	IncrementViewCount(ctx context.Context, postID uint) error

	// List queries posts with role-based visibility, filters and pagination.
	List(ctx context.Context, userRole string, userID uint, f ListFilter) ([]model.Post, int64, error)
	// MyPosts queries the current user's own posts (excluding rejected).
	MyPosts(ctx context.Context, userID uint, f ListFilter) ([]model.Post, int64, error)
	// Drafts queries draft posts; admins see all, other users only their own.
	Drafts(ctx context.Context, userRole string, userID uint, f ListFilter) ([]model.Post, int64, error)
	// UserPosts queries a user's posts with role-based visibility.
	UserPosts(ctx context.Context, authorID uint, userRole string, currentUserID uint, f ListFilter) ([]model.Post, int64, error)
	// Popular returns all published posts; scoring/sorting happens in the service.
	Popular(ctx context.Context) ([]model.Post, error)
	// Latest returns the most recent published posts.
	Latest(ctx context.Context, limit int) ([]model.Post, error)

	// Likes
	CountLikes(ctx context.Context, postID uint) (int64, error)
	FindLike(ctx context.Context, postID, userID uint) (*model.Like, error)
	CreateLike(ctx context.Context, like *model.Like) error
	// CreateLikeIfAbsent inserts a like unless one already exists for the
	// same post+user, returning whether a new row was created. It relies on
	// the composite unique index to stay correct under concurrency.
	CreateLikeIfAbsent(ctx context.Context, postID, userID uint) (bool, error)
	DeleteLike(ctx context.Context, like *model.Like) error

	// Comments
	CountComments(ctx context.Context, postID uint) (int64, error)
	DeleteCommentsByPostID(ctx context.Context, postID uint) error
	DeleteLikesByPostID(ctx context.Context, postID uint) error

	FindOrCreateTag(ctx context.Context, name string) (*model.Tag, error)
	ReplaceTagsAssociation(ctx context.Context, post *model.Post, tags []model.Tag) error
	ClearTagsAssociation(ctx context.Context, post *model.Post) error
	FindAllTags(ctx context.Context) ([]model.Tag, error)

	FindAllCategories(ctx context.Context) ([]model.Category, error)
	CreateCategory(ctx context.Context, category *model.Category) error

	FindUserByID(ctx context.Context, id uint) (*model.User, error)
	GetGuestViewSetting(ctx context.Context) bool

	ListModeration(ctx context.Context, status model.PostStatus, offset, limit int, search string) ([]model.Post, int64, error)

	// Batch queries to avoid N+1
	BatchCountLikesByPostIDs(ctx context.Context, postIDs []uint) (map[uint]int64, error)
	BatchCountCommentsByPostIDs(ctx context.Context, postIDs []uint) (map[uint]int64, error)
	BatchFindLikedPostIDs(ctx context.Context, postIDs []uint, userID uint) (map[uint]bool, error)
}

// gormRepository is the GORM-backed implementation of Repository.
type gormRepository struct {
	db *gorm.DB
}

// NewRepository creates a GORM-backed post repository.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) FindByID(ctx context.Context, id string) (*model.Post, error) {
	var post model.Post
	if err := r.db.WithContext(ctx).Preload("Author").Preload("Tags").First(&post, id).Error; err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *gormRepository) FindByIDPreloadTags(ctx context.Context, id string) (*model.Post, error) {
	var post model.Post
	if err := r.db.WithContext(ctx).Preload("Tags").First(&post, id).Error; err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *gormRepository) FindBySlug(ctx context.Context, slug string) (*model.Post, error) {
	var post model.Post
	if err := r.db.WithContext(ctx).
		Preload("Author").
		Preload("Tags").
		Where("slug = ?", slug).
		First(&post).Error; err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *gormRepository) FindBySlugExcludeID(ctx context.Context, slug string, excludeID uint) (*model.Post, error) {
	var post model.Post
	if err := r.db.WithContext(ctx).
		Where("slug = ? AND id != ?", slug, excludeID).
		First(&post).Error; err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *gormRepository) SlugExists(ctx context.Context, slug string) (bool, error) {
	err := r.db.WithContext(ctx).
		Model(&model.Post{}).
		Select("id").
		Where("slug = ?", slug).
		First(&model.Post{}).Error
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func (r *gormRepository) SlugExistsExcludeID(ctx context.Context, slug string, excludeID uint) (bool, error) {
	err := r.db.WithContext(ctx).
		Model(&model.Post{}).
		Select("id").
		Where("slug = ? AND id != ?", slug, excludeID).
		First(&model.Post{}).Error
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func (r *gormRepository) Create(ctx context.Context, post *model.Post) error {
	return r.db.WithContext(ctx).Create(post).Error
}

func (r *gormRepository) Save(ctx context.Context, post *model.Post) error {
	return r.db.WithContext(ctx).Save(post).Error
}

func (r *gormRepository) Delete(ctx context.Context, post *model.Post) error {
	return r.db.WithContext(ctx).Delete(post).Error
}

func (r *gormRepository) IncrementViewCount(ctx context.Context, postID uint) error {
	return r.db.WithContext(ctx).Model(&model.Post{}).Where("id = ?", postID).
		UpdateColumn("view_count", gorm.Expr("view_count + ?", 1)).Error
}

// baseQuery returns the post model query with author and tags preloaded.
func (r *gormRepository) baseQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Model(&model.Post{}).Preload("Author").Preload("Tags")
}

// listPage runs the shared count + paginate + order pattern for list queries.
func (r *gormRepository) listPage(query *gorm.DB, page, limit int) ([]model.Post, int64, error) {
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

// applyListVisibility narrows the query to posts visible to the given role.
func applyListVisibility(query *gorm.DB, userRole string, userID uint) *gorm.DB {
	switch userRole {
	case "", model.RoleGuest:
		return query.Where("status = ?", model.PostStatusPublished)
	case model.RoleContributor:
		return query.Where(
			query.Session(&gorm.Session{}).Where("status = ?", model.PostStatusPublished).
				Or("author_id = ? AND status != ?", userID, model.PostStatusRejected),
		)
	case model.RoleAuthor, model.RoleAdmin, model.RoleSuperAdmin:
		return query.Where("status != ?", model.PostStatusRejected)
	default:
		return query.Where("status = ?", model.PostStatusPublished)
	}
}

// applyUserPostsVisibility narrows a user's posts to those visible to the
// acting role; contributors see all of their own posts.
func applyUserPostsVisibility(query *gorm.DB, userRole string, authorID, currentUserID uint) *gorm.DB {
	switch userRole {
	case "", model.RoleGuest:
		return query.Where("status = ?", model.PostStatusPublished)
	case model.RoleContributor:
		if authorID != currentUserID {
			return query.Where("status = ?", model.PostStatusPublished)
		}
		return query.Where("status != ?", model.PostStatusRejected)
	default:
		return query.Where("status != ?", model.PostStatusRejected)
	}
}

func (r *gormRepository) List(ctx context.Context, userRole string, userID uint, f ListFilter) ([]model.Post, int64, error) {
	query := applyListVisibility(r.baseQuery(ctx), userRole, userID)

	// Category filter (by id or name)
	if f.CategoryID != "" {
		if cid, err := strconv.Atoi(f.CategoryID); err == nil {
			query = query.Where("category = ?", cid)
		} else {
			query = query.Where("category = ?", f.CategoryID)
		}
	}

	// Search filter
	if f.Search != "" {
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+f.Search+"%", "%"+f.Search+"%")
	}

	return r.listPage(query, f.Page, f.Limit)
}

func (r *gormRepository) MyPosts(ctx context.Context, userID uint, f ListFilter) ([]model.Post, int64, error) {
	query := r.baseQuery(ctx).Where("author_id = ? AND status != ?", userID, model.PostStatusRejected)

	if f.Status != "" {
		query = query.Where("status = ?", f.Status)
	}

	return r.listPage(query, f.Page, f.Limit)
}

func (r *gormRepository) Drafts(ctx context.Context, userRole string, userID uint, f ListFilter) ([]model.Post, int64, error) {
	query := r.baseQuery(ctx)

	if userRole != "" && model.IsAdmin(userRole) {
		// Admins and super admins can see all draft posts
		query = query.Where("status = ?", model.PostStatusDraft)
	} else {
		// Other users can only see their own draft posts
		query = query.Where("author_id = ? AND status = ?", userID, model.PostStatusDraft)
	}

	return r.listPage(query, f.Page, f.Limit)
}

func (r *gormRepository) UserPosts(ctx context.Context, authorID uint, userRole string, currentUserID uint, f ListFilter) ([]model.Post, int64, error) {
	query := applyUserPostsVisibility(r.baseQuery(ctx).Where("author_id = ?", authorID), userRole, authorID, currentUserID)
	return r.listPage(query, f.Page, f.Limit)
}

func (r *gormRepository) Popular(ctx context.Context) ([]model.Post, error) {
	var posts []model.Post
	err := r.baseQuery(ctx).Where("status = ?", model.PostStatusPublished).Find(&posts).Error
	return posts, err
}

func (r *gormRepository) Latest(ctx context.Context, limit int) ([]model.Post, error) {
	var posts []model.Post
	err := r.baseQuery(ctx).Where("status = ?", model.PostStatusPublished).
		Order("created_at DESC").Limit(limit).Find(&posts).Error
	return posts, err
}

func (r *gormRepository) CountLikes(ctx context.Context, postID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Like{}).Where("post_id = ?", postID).Count(&count).Error
	return count, err
}

func (r *gormRepository) FindLike(ctx context.Context, postID, userID uint) (*model.Like, error) {
	var like model.Like
	if err := r.db.WithContext(ctx).Where("post_id = ? AND user_id = ?", postID, userID).First(&like).Error; err != nil {
		return nil, err
	}
	return &like, nil
}

func (r *gormRepository) CreateLike(ctx context.Context, like *model.Like) error {
	return r.db.WithContext(ctx).Create(like).Error
}

func (r *gormRepository) CreateLikeIfAbsent(ctx context.Context, postID, userID uint) (bool, error) {
	like := model.Like{PostID: postID, UserID: userID}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "post_id"}, {Name: "user_id"}},
		DoNothing: true,
	}).Create(&like)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *gormRepository) DeleteLike(ctx context.Context, like *model.Like) error {
	return r.db.WithContext(ctx).Delete(like).Error
}

func (r *gormRepository) CountComments(ctx context.Context, postID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Comment{}).Where("post_id = ?", postID).Count(&count).Error
	return count, err
}

func (r *gormRepository) DeleteCommentsByPostID(ctx context.Context, postID uint) error {
	return r.db.WithContext(ctx).Where("post_id = ?", postID).Delete(&model.Comment{}).Error
}

func (r *gormRepository) DeleteLikesByPostID(ctx context.Context, postID uint) error {
	return r.db.WithContext(ctx).Where("post_id = ?", postID).Delete(&model.Like{}).Error
}

func (r *gormRepository) FindOrCreateTag(ctx context.Context, name string) (*model.Tag, error) {
	var tag model.Tag
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&tag).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			tag = model.Tag{Name: name}
			if err := r.db.WithContext(ctx).Create(&tag).Error; err != nil {
				// A concurrent request can create the tag between the lookup
				// and the insert; the unique index then rejects our create.
				// Recover by returning the winner instead of failing.
				if errors.Is(err, gorm.ErrDuplicatedKey) {
					if retryErr := r.db.WithContext(ctx).Where("name = ?", name).First(&tag).Error; retryErr != nil {
						return nil, retryErr
					}
					return &tag, nil
				}
				return nil, err
			}
			return &tag, nil
		}
		return nil, err
	}
	return &tag, nil
}

func (r *gormRepository) ReplaceTagsAssociation(ctx context.Context, post *model.Post, tags []model.Tag) error {
	return r.db.WithContext(ctx).Model(post).Association("Tags").Replace(tags)
}

func (r *gormRepository) ClearTagsAssociation(ctx context.Context, post *model.Post) error {
	return r.db.WithContext(ctx).Model(post).Association("Tags").Clear()
}

func (r *gormRepository) FindAllTags(ctx context.Context) ([]model.Tag, error) {
	var tags []model.Tag
	if err := r.db.WithContext(ctx).Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *gormRepository) FindAllCategories(ctx context.Context) ([]model.Category, error) {
	var categories []model.Category
	if err := r.db.WithContext(ctx).Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

func (r *gormRepository) CreateCategory(ctx context.Context, category *model.Category) error {
	// The unique index on name is the source of truth for duplicates: insert
	// directly and translate the constraint violation (GORM maps driver
	// errors to gorm.ErrDuplicatedKey when opened with TranslateError) into
	// ErrDuplicateName, which the handler renders as a 409. A select-then-
	// insert pre-check would race: two concurrent creates could both pass the
	// check and the loser would surface as an unmapped 500.
	if err := r.db.WithContext(ctx).Create(category).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrDuplicateName
		}
		return err
	}
	return nil
}

func (r *gormRepository) FindUserByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormRepository) GetGuestViewSetting(ctx context.Context) bool {
	var config model.GeneralSettings
	if err := r.db.WithContext(ctx).First(&config).Error; err != nil {
		return true
	}
	return config.AllowGuestViewPosts
}

func (r *gormRepository) ListModeration(ctx context.Context, status model.PostStatus, offset, limit int, search string) ([]model.Post, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.Post{}).
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
		Offset(offset).
		Limit(limit).
		Find(&posts).Error; err != nil {
		return nil, 0, err
	}

	return posts, total, nil
}

func (r *gormRepository) BatchCountLikesByPostIDs(ctx context.Context, postIDs []uint) (map[uint]int64, error) {
	if len(postIDs) == 0 {
		return make(map[uint]int64), nil
	}
	type result struct {
		PostID uint
		Count  int64
	}
	var results []result
	err := r.db.WithContext(ctx).Model(&model.Like{}).
		Select("post_id, COUNT(*) as count").
		Where("post_id IN ?", postIDs).
		Group("post_id").
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[uint]int64, len(postIDs))
	for _, r := range results {
		counts[r.PostID] = r.Count
	}
	return counts, nil
}

func (r *gormRepository) BatchCountCommentsByPostIDs(ctx context.Context, postIDs []uint) (map[uint]int64, error) {
	if len(postIDs) == 0 {
		return make(map[uint]int64), nil
	}
	type result struct {
		PostID uint
		Count  int64
	}
	var results []result
	err := r.db.WithContext(ctx).Model(&model.Comment{}).
		Select("post_id, COUNT(*) as count").
		Where("post_id IN ?", postIDs).
		Group("post_id").
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[uint]int64, len(postIDs))
	for _, r := range results {
		counts[r.PostID] = r.Count
	}
	return counts, nil
}

func (r *gormRepository) BatchFindLikedPostIDs(ctx context.Context, postIDs []uint, userID uint) (map[uint]bool, error) {
	if len(postIDs) == 0 || userID == 0 {
		return make(map[uint]bool), nil
	}
	var likedPostIDs []uint
	err := r.db.WithContext(ctx).Model(&model.Like{}).
		Where("post_id IN ? AND user_id = ?", postIDs, userID).
		Pluck("post_id", &likedPostIDs).Error
	if err != nil {
		return nil, err
	}
	liked := make(map[uint]bool, len(likedPostIDs))
	for _, id := range likedPostIDs {
		liked[id] = true
	}
	return liked, nil
}
