package mock

import (
	"context"
	"io"
	"sync"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/repository"
	"feedchamber/internal/storage"
)

// ── UploadRepository ────────────────────────────────────────────────────────

type UploadRepository struct {
	mu        sync.Mutex
	uploads   map[string]*model.Upload
	findErr   error
	deleteErr error
}

func NewUploadRepository() *UploadRepository {
	return &UploadRepository{uploads: make(map[string]*model.Upload)}
}

func (m *UploadRepository) Create(_ context.Context, u *model.Upload) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.uploads[u.ID] = u
	return nil
}

func (m *UploadRepository) FindByID(_ context.Context, id string) (*model.Upload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.findErr != nil {
		return nil, m.findErr
	}
	u, ok := m.uploads[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (m *UploadRepository) FindByKey(_ context.Context, key string) (*model.Upload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.findErr != nil {
		return nil, m.findErr
	}
	for _, u := range m.uploads {
		if u.Key == key {
			return u, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (m *UploadRepository) ListByUser(_ context.Context, userID string, offset, limit int) ([]model.Upload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []model.Upload
	for _, u := range m.uploads {
		if u.UploaderID == userID {
			result = append(result, *u)
		}
	}
	if offset > len(result) {
		return nil, nil
	}
	result = result[offset:]
	if limit < len(result) {
		result = result[:limit]
	}
	return result, nil
}

func (m *UploadRepository) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.uploads, id)
	return nil
}

func (m *UploadRepository) SetFindErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.findErr = err
}

func (m *UploadRepository) SetDeleteErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteErr = err
}

// ── StorageProvider ───────────────────────────────────────────────────────────

type StorageProvider struct {
	PutErr      error
	DeleteErr   error
	PublicURLFn func(key string) string
	PutKeys     []string
	DeleteKeys  []string
}

func (s *StorageProvider) Put(_ context.Context, key string, _ io.Reader, _ int64, _ string) error {
	s.PutKeys = append(s.PutKeys, key)
	return s.PutErr
}

func (s *StorageProvider) Get(_ context.Context, key string) (io.ReadCloser, error) {
	return nil, nil
}

func (s *StorageProvider) Delete(_ context.Context, key string) error {
	s.DeleteKeys = append(s.DeleteKeys, key)
	return s.DeleteErr
}

func (s *StorageProvider) List(_ context.Context, _ string) ([]storage.ObjectInfo, error) {
	return nil, nil
}

func (s *StorageProvider) PresignPostURL(_ context.Context, _ string, _ string, _ int64, _ time.Duration) (*storage.PresignedPost, error) {
	return &storage.PresignedPost{URL: "https://example.com/presign"}, nil
}

func (s *StorageProvider) PublicURL(key string) string {
	if s.PublicURLFn != nil {
		return s.PublicURLFn(key)
	}
	return "https://example.com/" + key
}

// ── SettingService ───────────────────────────────────────────────────────────

type SettingService struct {
	settings map[string]string
	mu       sync.Mutex
}

func NewSettingService() *SettingService {
	return &SettingService{settings: make(map[string]string)}
}

func (s *SettingService) WithSetting(key, value string) *SettingService {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings[key] = value
	return s
}

func (s *SettingService) Get(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.settings[key]; ok {
		return v, nil
	}
	return "", repository.ErrNotFound
}

func (s *SettingService) GetAll(_ context.Context) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make(map[string]string)
	for k, v := range s.settings {
		cp[k] = v
	}
	return cp, nil
}

func (s *SettingService) Set(_ context.Context, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings[key] = value
	return nil
}

func (s *SettingService) BatchSet(_ context.Context, settings map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range settings {
		s.settings[k] = v
	}
	return nil
}

func (s *SettingService) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.settings, key)
	return nil
}

// ── ReportRepository ─────────────────────────────────

type ReportRepository struct {
	mu      sync.RWMutex
	reports map[string]*model.Report
	findErr error
}

func NewReportRepository() *ReportRepository {
	return &ReportRepository{reports: make(map[string]*model.Report)}
}

func (m *ReportRepository) Create(_ context.Context, r *model.Report) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reports[r.ID] = r
	return nil
}

func (m *ReportRepository) FindByID(_ context.Context, id string) (*model.Report, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.reports[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return r, nil
}

func (m *ReportRepository) FindPendingByTarget(_ context.Context, targetType, targetID string) (*model.Report, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, r := range m.reports {
		if r.TargetType == targetType && r.TargetID == targetID && r.Status == "pending" {
			return r, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (m *ReportRepository) IncrementCount(_ context.Context, id, reporterID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.reports[id]
	if !ok {
		return repository.ErrNotFound
	}
	r.Count++
	r.ReporterID = reporterID
	r.UpdatedAt = time.Now()
	return nil
}

func (m *ReportRepository) List(_ context.Context, offset, limit int) ([]model.Report, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := make([]model.Report, 0, len(m.reports))
	for _, r := range m.reports {
		all = append(all, *r)
	}
	if offset >= len(all) {
		return nil, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

func (m *ReportRepository) ListByStatus(_ context.Context, status string, offset, limit int) ([]model.Report, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var filtered []model.Report
	for _, r := range m.reports {
		if r.Status == status {
			filtered = append(filtered, *r)
		}
	}
	if offset >= len(filtered) {
		return nil, nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[offset:end], nil
}

func (m *ReportRepository) Count(_ context.Context) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.reports)), nil
}

func (m *ReportRepository) CountByStatus(_ context.Context, status string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var count int64
	for _, r := range m.reports {
		if r.Status == status {
			count++
		}
	}
	return count, nil
}

func (m *ReportRepository) UpdateStatus(_ context.Context, id, status, actorID, action, note string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.reports[id]
	if !ok {
		return repository.ErrNotFound
	}
	r.Status = status
	r.Action = action
	r.HandledBy = actorID
	r.Note = note
	now := time.Now()
	r.HandledAt = &now
	r.UpdatedAt = now
	return nil
}

func (m *ReportRepository) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.reports, id)
	return nil
}

func (m *ReportRepository) SetFindErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.findErr = err
}

// ── ContentRepository (minimal) ──────────────────────

type ContentRepository struct {
	mu       sync.RWMutex
	contents map[string]*model.Content
	findErr  error
}

func NewContentRepository() *ContentRepository {
	return &ContentRepository{contents: make(map[string]*model.Content)}
}

func (m *ContentRepository) Create(_ context.Context, c *model.Content) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.contents[c.ID] = c
	return nil
}

func (m *ContentRepository) FindByID(_ context.Context, id string) (*model.Content, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.contents[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return c, nil
}

func (m *ContentRepository) SetFindErr(err error) { m.mu.Lock(); defer m.mu.Unlock(); m.findErr = err }

// stub out the remaining interface methods:
func (m *ContentRepository) ListByRoot(_ context.Context, _ string, _, _ int, _, _ *time.Time) ([]model.ContentListItem, error) {
	return nil, nil
}
func (m *ContentRepository) CountByRoot(_ context.Context, _ string, _, _ *time.Time) (int, error) {
	return 0, nil
}
func (m *ContentRepository) ListByRootWithTotal(_ context.Context, _ string, _, _ int, _, _ *time.Time) ([]model.ContentListItem, int, error) {
	return nil, 0, nil
}
func (m *ContentRepository) CountUnreadByRoot(_ context.Context, _ string, _ time.Time) (int, error) {
	return 0, nil
}
func (m *ContentRepository) ListByParent(_ context.Context, _ string, _, _ int, _, _ *time.Time) ([]model.ContentListItem, error) {
	return nil, nil
}
func (m *ContentRepository) CountByParent(_ context.Context, _ string, _, _ *time.Time) (int, error) {
	return 0, nil
}
func (m *ContentRepository) ListByParentWithTotal(_ context.Context, _ string, _, _ int, _, _ *time.Time) ([]model.ContentListItem, int, error) {
	return nil, 0, nil
}
func (m *ContentRepository) ListAll(_ context.Context, _, _ int) ([]model.ContentListItem, error) {
	return nil, nil
}
func (m *ContentRepository) CountAll(_ context.Context) (int, error) { return 0, nil }
func (m *ContentRepository) ListAllWithTotal(_ context.Context, _, _ int) ([]model.ContentListItem, int, error) {
	return nil, 0, nil
}
func (m *ContentRepository) ListTrash(_ context.Context, _, _ int) ([]model.ContentListItem, int, error) {
	return nil, 0, nil
}
func (m *ContentRepository) Update(_ context.Context, c *model.Content) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c != nil {
		m.contents[c.ID] = c
	}
	return nil
}
func (m *ContentRepository) Delete(_ context.Context, _ string) error             { return nil }
func (m *ContentRepository) Restore(_ context.Context, _ string) error            { return nil }
func (m *ContentRepository) IncrementLikeCount(_ context.Context, _ string) error { return nil }
func (m *ContentRepository) DecrementLikeCount(_ context.Context, _ string) error { return nil }

// ── PostRepository (minimal) ─────────────────────────

type PostRepository struct {
	mu             sync.RWMutex
	posts          map[string]*model.Post // keyed by post.ID
	byContentID    map[string]*model.Post // keyed by post.ContentID
	LastListFilter repository.PostListFilter
}

func NewPostRepository() *PostRepository {
	return &PostRepository{
		posts:       make(map[string]*model.Post),
		byContentID: make(map[string]*model.Post),
	}
}

func (m *PostRepository) Create(_ context.Context, p *model.Post) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.posts[p.ID] = p
	m.byContentID[p.ContentID] = p
	return nil
}

func (m *PostRepository) FindByID(_ context.Context, id string) (*model.Post, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.posts[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return p, nil
}

func (m *PostRepository) FindByIDs(_ context.Context, ids []string) ([]*model.Post, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var posts []*model.Post
	for _, id := range ids {
		if p, ok := m.posts[id]; ok {
			posts = append(posts, p)
		}
	}
	return posts, nil
}

func (m *PostRepository) FindByIDsDetails(_ context.Context, ids []string) ([]model.PostListItem, error) {
	return []model.PostListItem{}, nil
}

func (m *PostRepository) FindByContentID(_ context.Context, contentID string) (*model.Post, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.byContentID[contentID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return p, nil
}

// stub remaining interface methods (return zero/nil):
func (m *PostRepository) List(_ context.Context, _, _ int) ([]model.Post, error) { return nil, nil }
func (m *PostRepository) ListSitemapEntries(_ context.Context, _, _ int, _ []string) ([]repository.SitemapEntry, error) {
	return nil, nil
}
func (m *PostRepository) CountSitemapEntries(_ context.Context, _ []string) (int, error) {
	return 0, nil
}
func (m *PostRepository) ListTrash(_ context.Context, _, _ int) ([]model.PostListItem, int, error) {
	return nil, 0, nil
}
func (m *PostRepository) Restore(_ context.Context, _ string) error { return nil }
func (m *PostRepository) CreateWithContent(_ context.Context, p *model.Post, c *model.Content) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p != nil {
		m.posts[p.ID] = p
		if p.ContentID != "" {
			m.byContentID[p.ContentID] = p
		}
	}
	return nil
}
func (m *PostRepository) Update(_ context.Context, p *model.Post) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p != nil {
		m.posts[p.ID] = p
		if p.ContentID != "" {
			m.byContentID[p.ContentID] = p
		}
	}
	return nil
}
func (m *PostRepository) Delete(_ context.Context, _ string) error                        { return nil }
func (m *PostRepository) IncrementViewCount(_ context.Context, _ string) error            { return nil }
func (m *PostRepository) IncrementViewCountBy(_ context.Context, _ string, _ int64) error { return nil }
func (m *PostRepository) IncrementCommentCount(_ context.Context, _ string) error         { return nil }
func (m *PostRepository) DecrementCommentCount(_ context.Context, _ string) error         { return nil }
func (m *PostRepository) IncrementLikeCount(_ context.Context, _ string) error            { return nil }
func (m *PostRepository) DecrementLikeCount(_ context.Context, _ string) error            { return nil }
func (m *PostRepository) IncrementFavoriteCount(_ context.Context, _ string) error        { return nil }
func (m *PostRepository) DecrementFavoriteCount(_ context.Context, _ string) error        { return nil }
func (m *PostRepository) ListWithFilter(_ context.Context, _ repository.PostListFilter, _, _ int) ([]model.PostListItem, error) {
	return nil, nil
}
func (m *PostRepository) CountWithFilter(_ context.Context, _ repository.PostListFilter) (int, error) {
	return 0, nil
}
func (m *PostRepository) ListWithTotal(_ context.Context, filter repository.PostListFilter, _, _ int) ([]model.PostListItem, int, error) {
	m.mu.Lock()
	m.LastListFilter = filter
	m.mu.Unlock()
	return nil, 0, nil
}
func (m *PostRepository) FindByIDWithDetails(_ context.Context, _ string) (*model.PostListItem, error) {
	return nil, nil
}
func (m *PostRepository) FindDetailByID(_ context.Context, _ string) (*model.PostDetail, error) {
	return nil, nil
}
func (m *PostRepository) Search(_ context.Context, _ string, _, _ int) ([]model.PostListItem, error) {
	return nil, nil
}
func (m *PostRepository) CountSearch(_ context.Context, _ string) (int, error) { return 0, nil }
func (m *PostRepository) SearchWithTotal(_ context.Context, _ string, _, _ int) ([]model.PostListItem, int, error) {
	return nil, 0, nil
}
func (m *PostRepository) SearchWithTotalFilter(_ context.Context, _ string, _ repository.PostListFilter, _, _ int) ([]model.PostListItem, int, error) {
	return nil, 0, nil
}
func (m *PostRepository) CountBySpaceID(_ context.Context, _ string) (int, error) { return 0, nil }
func (m *PostRepository) TogglePinned(_ context.Context, _ string) (*model.Post, error) {
	return &model.Post{}, nil
}
func (m *PostRepository) ToggleLocked(_ context.Context, _ string) (*model.Post, error) {
	return &model.Post{}, nil
}
func (m *PostRepository) TouchActive(_ context.Context, id string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.posts[id]; ok {
		p.LastActiveAt = at
		p.UpdatedAt = at
	}
	return nil
}
func (m *PostRepository) SetPin(_ context.Context, _ string, _ repository.SetPinInput) (*model.Post, error) {
	return &model.Post{}, nil
}
func (m *PostRepository) Unpin(_ context.Context, _ string) (*model.Post, error) {
	return &model.Post{}, nil
}
func (m *PostRepository) ExpirePins(_ context.Context) (int64, error) { return 0, nil }
func (m *PostRepository) ListPinnedBySpace(_ context.Context, _ string, _, _ int, _ repository.PostListFilter) ([]model.PostListItem, int64, error) {
	return nil, 0, nil
}

// ── UserRepository (minimal) ─────────────────────────

type UserRepository struct {
	mu    sync.RWMutex
	users map[string]*model.User // keyed by UUID
}

func NewUserRepository() *UserRepository {
	return &UserRepository{users: make(map[string]*model.User)}
}

func (m *UserRepository) Create(_ context.Context, u *model.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.ID] = u
	return nil
}

func (m *UserRepository) FindByID(_ context.Context, uuid string) (*model.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[uuid]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

// stub remaining interface methods:
func (m *UserRepository) FindByEmail(_ context.Context, _ string) (*model.User, error) {
	return nil, repository.ErrNotFound
}
func (m *UserRepository) FindByUsername(_ context.Context, _ string) (*model.User, error) {
	return nil, repository.ErrNotFound
}
func (m *UserRepository) FindByIdentifier(_ context.Context, _ string) (*model.User, error) {
	return nil, repository.ErrNotFound
}
func (m *UserRepository) List(_ context.Context, _, _ int) ([]model.User, error) { return nil, nil }
func (m *UserRepository) Search(_ context.Context, _, _ string, _, _ int) ([]model.User, error) {
	return nil, nil
}
func (m *UserRepository) CountUsersByPermission(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (m *UserRepository) GetActivityStats(_ context.Context, _ string, _ int) (map[string]int, error) {
	return nil, nil
}
func (m *UserRepository) CountPosts(_ context.Context, _ string) (int64, error) { return 0, nil }
func (m *UserRepository) CountCommentsByUser(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (m *UserRepository) CountLikesByUser(_ context.Context, _ string) (int64, error) { return 0, nil }
func (m *UserRepository) CountLikesReceived(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (m *UserRepository) IncrPoints(_ context.Context, id string, delta int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return repository.ErrNotFound
	}
	u.TotalXP += delta
	return nil
}
func (m *UserRepository) ListByPoints(_ context.Context, _, _ int) ([]model.User, error) {
	return nil, nil
}
func (m *UserRepository) ListByCheckIns(_ context.Context, _, _ int) ([]model.User, error) {
	return nil, nil
}
func (m *UserRepository) ListByPostCount(_ context.Context, _, _ int) ([]model.User, error) {
	return nil, nil
}
func (m *UserRepository) Count(_ context.Context) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.users)), nil
}
func (m *UserRepository) CountDistinctPostAuthors(_ context.Context) (int64, error) { return 0, nil }
func (m *UserRepository) Update(_ context.Context, u *model.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.ID] = u
	return nil
}
func (m *UserRepository) Delete(_ context.Context, _ string) error { return nil }

// ── TopicRead ───────────────────────────────────────────────────────────────

type TopicReadRepository struct {
	mu     sync.RWMutex
	states map[string]*model.TopicReadState // key: userID+"\x00"+postID
}

func NewTopicReadRepository() *TopicReadRepository {
	return &TopicReadRepository{states: make(map[string]*model.TopicReadState)}
}

func topicReadKey(userID, postID string) string { return userID + "\x00" + postID }

func (m *TopicReadRepository) Upsert(_ context.Context, userID, postID, lastContentID string, lastReadAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := topicReadKey(userID, postID)
	if existing, ok := m.states[key]; ok {
		existing.LastReadAt = lastReadAt
		existing.LastContentID = lastContentID
		return nil
	}
	m.states[key] = &model.TopicReadState{
		ID:            "tr-" + key,
		UserID:        userID,
		PostID:        postID,
		LastReadAt:    lastReadAt,
		LastContentID: lastContentID,
	}
	return nil
}

func (m *TopicReadRepository) FindByUserAndPost(_ context.Context, userID, postID string) (*model.TopicReadState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st, ok := m.states[topicReadKey(userID, postID)]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *st
	return &cp, nil
}

func (m *TopicReadRepository) ListByUserAndPosts(_ context.Context, userID string, postIDs []string) ([]model.TopicReadState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.TopicReadState, 0, len(postIDs))
	for _, pid := range postIDs {
		if st, ok := m.states[topicReadKey(userID, pid)]; ok {
			out = append(out, *st)
		}
	}
	return out, nil
}

func (m *TopicReadRepository) ListByUser(_ context.Context, userID string) ([]model.TopicReadState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.TopicReadState, 0)
	for key, st := range m.states {
		if key[:len(userID)] == userID {
			out = append(out, *st)
		}
	}
	return out, nil
}

// ── MotionRepository ────────────────────────────────────────────────────────

type MotionRepository struct {
	mu      sync.RWMutex
	motions map[string]*model.Motion
	votes   map[string]*model.MotionVote // key: motionID+"\x00"+voterID
}

func NewMotionRepository() *MotionRepository {
	return &MotionRepository{
		motions: make(map[string]*model.Motion),
		votes:   make(map[string]*model.MotionVote),
	}
}

func (m *MotionRepository) Create(_ context.Context, mo *model.Motion) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.motions[mo.ID] = mo
	return nil
}

func (m *MotionRepository) FindByID(_ context.Context, id string) (*model.Motion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	mo, ok := m.motions[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return mo, nil
}

func (m *MotionRepository) FindPendingByTarget(_ context.Context, targetType, targetID string) (*model.Motion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, mo := range m.motions {
		if mo.TargetType == targetType && mo.TargetID == targetID && mo.Status == model.MotionStatusPending {
			return mo, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (m *MotionRepository) List(_ context.Context, status, targetType string, offset, limit int) ([]model.Motion, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := make([]model.Motion, 0, len(m.motions))
	for _, mo := range m.motions {
		if status != "" && mo.Status != status {
			continue
		}
		if targetType != "" && mo.TargetType != targetType {
			continue
		}
		all = append(all, *mo)
	}
	total := int64(len(all))
	if offset >= len(all) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], total, nil
}

func (m *MotionRepository) CountByStatus(_ context.Context, status string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var n int64
	for _, mo := range m.motions {
		if mo.Status == status {
			n++
		}
	}
	return n, nil
}

func (m *MotionRepository) AddVote(_ context.Context, v *model.MotionVote) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := v.MotionID + "\x00" + v.VoterID
	if _, exists := m.votes[key]; exists {
		return repository.ErrDuplicate
	}
	m.votes[key] = v
	return nil
}

func (m *MotionRepository) FindVote(_ context.Context, motionID, voterID string) (*model.MotionVote, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.votes[motionID+"\x00"+voterID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return v, nil
}

func (m *MotionRepository) IncrementTally(_ context.Context, motionID string, approveDelta, rejectDelta float64, voteDelta int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mo, ok := m.motions[motionID]
	if !ok {
		return repository.ErrNotFound
	}
	mo.ApproveWeight += approveDelta
	mo.RejectWeight += rejectDelta
	mo.VoteCount += voteDelta
	return nil
}

func (m *MotionRepository) UpdateStatus(_ context.Context, id, status string, resolvedAt *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mo, ok := m.motions[id]
	if !ok {
		return repository.ErrNotFound
	}
	mo.Status = status
	mo.ResolvedAt = resolvedAt
	return nil
}

func (m *MotionRepository) MarkAbuse(_ context.Context, id string, abuse bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mo, ok := m.motions[id]
	if !ok {
		return repository.ErrNotFound
	}
	mo.AbuseFlag = abuse
	return nil
}

func (m *MotionRepository) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.motions, id)
	return nil
}

// ── SpaceRepository (minimal, for dark-zone resolution) ─────────────────────

type SpaceRepository struct {
	mu     sync.RWMutex
	spaces map[string]*model.Space
}

func NewSpaceRepository() *SpaceRepository {
	return &SpaceRepository{spaces: make(map[string]*model.Space)}
}

func (m *SpaceRepository) List(_ context.Context) ([]model.Space, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.Space, 0, len(m.spaces))
	for _, s := range m.spaces {
		out = append(out, *s)
	}
	return out, nil
}

func (m *SpaceRepository) FindByID(_ context.Context, id string) (*model.Space, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.spaces[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return s, nil
}

func (m *SpaceRepository) FindBySlug(_ context.Context, slug string) (*model.Space, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.spaces {
		if s.Slug == slug {
			return s, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (m *SpaceRepository) Create(_ context.Context, s *model.Space) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spaces[s.ID] = s
	return nil
}

func (m *SpaceRepository) Update(_ context.Context, s *model.Space) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spaces[s.ID] = s
	return nil
}

func (m *SpaceRepository) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.spaces, id)
	return nil
}

func (m *SpaceRepository) ToggleActive(_ context.Context, _ string) (*model.Space, error) {
	return nil, repository.ErrNotFound
}
func (m *SpaceRepository) FindChildren(_ context.Context, _ string) ([]model.Space, error) {
	return nil, nil
}
func (m *SpaceRepository) FindAncestors(_ context.Context, _ string) ([]model.Space, error) {
	return nil, nil
}
func (m *SpaceRepository) FindDescendants(_ context.Context, _ string) ([]model.Space, error) {
	return nil, nil
}
func (m *SpaceRepository) FindDescendantIDs(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (m *SpaceRepository) HasChildren(_ context.Context, _ string) (bool, error)    { return false, nil }
func (m *SpaceRepository) CountChildren(_ context.Context, _ string) (int, error)   { return 0, nil }
func (m *SpaceRepository) ListRoots(_ context.Context) ([]model.Space, error)       { return nil, nil }
func (m *SpaceRepository) Exists(_ context.Context, _ string) (bool, error)         { return false, nil }
func (m *SpaceRepository) BatchUpdateSortOrder(_ context.Context, _ []string) error { return nil }
func (m *SpaceRepository) BatchUpdateDepthPath(_ context.Context, _ map[string]struct {
	Depth int
	Path  string
}) error {
	return nil
}

func (m *SpaceRepository) SetSystemFlags(_ context.Context, id string, system, recommendable bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.spaces[id]
	if !ok {
		return repository.ErrNotFound
	}
	s.System = system
	s.Recommendable = recommendable
	return nil
}

func (m *SpaceRepository) BackfillRecommendable(_ context.Context) error { return nil }

func (m *MotionRepository) ListExpiredPending(_ context.Context, before time.Time, limit int) ([]model.Motion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []model.Motion
	for _, mo := range m.motions {
		if mo.Status == model.MotionStatusPending && !mo.CreatedAt.After(before) {
			out = append(out, *mo)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}
