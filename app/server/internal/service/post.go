package service

import (
	"context"
	"errors"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/eventbus"
	"feedchamber/internal/pkg/kvstore"
	"feedchamber/internal/pkg/readcache"
	"feedchamber/internal/repository"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type viewerIDCtxKey struct{}

// WithViewerID attaches the current viewer user ID for list filtering.
func WithViewerID(ctx context.Context, userID string) context.Context {
	if userID == "" {
		return ctx
	}
	return context.WithValue(ctx, viewerIDCtxKey{}, userID)
}

type viewerRoleCtxKey struct{}

// WithViewerRole attaches the current viewer role for visibility checks.
func WithViewerRole(ctx context.Context, role string) context.Context {
	if role == "" {
		return ctx
	}
	return context.WithValue(ctx, viewerRoleCtxKey{}, role)
}

type listViewCtxKey struct{}

// WithListView attaches the requested user-aware list view to the request context.
func WithListView(ctx context.Context, listView string) context.Context {
	if listView == "" {
		return ctx
	}
	return context.WithValue(ctx, listViewCtxKey{}, listView)
}

func isPublicPostStatus(status string) bool {
	return status == model.PostStatusApproved || status == model.PostStatusClosed
}

func (s *postService) canViewNonPublic(ctx context.Context) bool {
	role, _ := ctx.Value(viewerRoleCtxKey{}).(string)
	if s.perms != nil {
		return s.perms.HasPermission(role, model.PermReportManage)
	}
	switch role {
	case "admin", "moderator", "super_admin":
		return true
	default:
		return false
	}
}

// canViewPost reports whether the current viewer may read a post in the given
// status. Public statuses are open to everyone; non-public ones require the
// review permission (report:manage) OR authorship — the author must be able to
// read their own pending post instead of getting a 404 for something they just
// created successfully.
func (s *postService) canViewPost(ctx context.Context, status, authorID string) bool {
	if isPublicPostStatus(status) || s.canViewNonPublic(ctx) {
		return true
	}
	viewerID, _ := ctx.Value(viewerIDCtxKey{}).(string)
	return viewerID != "" && authorID != "" && viewerID == authorID
}

// viewerVisibility 解析当前访问者的等级与登录态：游客一律 TL0。
func (s *postService) viewerVisibility(ctx context.Context) (viewerID string, level int, loggedIn bool) {
	viewerID, _ = ctx.Value(viewerIDCtxKey{}).(string)
	if viewerID == "" || s.users == nil {
		return "", 0, false
	}
	u, err := s.users.FindByID(ctx, viewerID)
	if err != nil || u == nil {
		return viewerID, 0, true
	}
	return viewerID, u.TrustLevel, true
}

// visibilityIndex 构建版面有效可见性索引（含祖先继承）。失败时返回错误而不是
// 空索引：把受限版面当公开会直接泄露内容，宁可让请求 500。
func (s *postService) visibilityIndex(ctx context.Context) (VisibilityIndex, error) {
	if s.spaceRepo == nil {
		return VisibilityIndex{}, nil
	}
	spaces, err := s.spaceRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	return BuildVisibilityIndex(spaces), nil
}

// applyVisibility 把可见性谓词写进列表/搜索过滤器。有审核权限的后台调用者
// 不受可见性限制（否则 /admin/posts 看不到受限帖，无法治理）。
func (s *postService) applyVisibility(ctx context.Context, filter *repository.PostListFilter) error {
	if s.canViewNonPublic(ctx) {
		return nil
	}
	idx, err := s.visibilityIndex(ctx)
	if err != nil {
		return err
	}
	viewerID, level, loggedIn := s.viewerVisibility(ctx)
	filter.EnforceVisibility = true
	filter.ViewerID = viewerID
	filter.ViewerLevel = level
	filter.VisibleSpaceIDs = idx.VisibleSpaceIDs(level, loggedIn)
	return nil
}

// visibleToViewer 是单篇/批量读取的统一判定；details 出口共用它，避免各写一套。
func visibleToViewer(item model.PostListItem, viewerID string, level int, idx VisibilityIndex) bool {
	if item.AuthorID != "" && item.AuthorID == viewerID {
		return true
	}
	return level >= idx.PostRequiredLevel(item.SpaceID, item.MinLevel)
}

// routeExiledAuthor 把当前处于软隔离（exile）状态的作者的新帖路由到暗区。
//
// 依据 design D3：exile 通过后置 users.exiled_until，其"新发帖在 service 层路由到
// dark-zone"。返回值直接用于创建帖子；任何查询失败都保持原 spaceID 不变（软隔离
// 属治理增强，不能因为暗区缺失就让用户发不出帖）。
func (s *postService) routeExiledAuthor(ctx context.Context, authorID, spaceID string) (routedSpaceID string, routed bool) {
	if s.users == nil || s.spaceRepo == nil {
		return spaceID, false
	}
	u, err := s.users.FindByID(ctx, authorID)
	if err != nil || u == nil || u.ExiledUntil == nil {
		return spaceID, false
	}
	// 过期即视为已解除（翻案也会把该字段清空）
	if !u.ExiledUntil.IsZero() && u.ExiledUntil.Before(time.Now()) {
		return spaceID, false
	}
	dz, err := s.spaceRepo.FindBySlug(ctx, DarkZoneSlug)
	if err != nil || dz == nil {
		return spaceID, false
	}
	return dz.ID, true
}

type postService struct {
	posts         repository.PostRepository
	contents      repository.ContentRepository
	topicReads    repository.TopicReadRepository
	banSvc        BannedService
	txMgr         db.TransactionManager
	kv            kvstore.Store
	settingSvc    SettingService
	spaceRepo     repository.SpaceRepository
	perms         PermissionChecker
	moderationSvc ModerationService
	users         repository.UserRepository
	bus           *eventbus.EventBus
	blockSvc      UserBlockService
	badgeSvc      BadgeService
	revisions     repository.RevisionRepository
	trustSvc      TrustService // optional
	gateSvc       GateService  // optional（门控规则引擎）

	// readCache 缓存**匿名**列表结果（见 List 开头的可缓存判定）。
	// 登录态列表带已读状态/拉黑过滤，按人不同，不能进共享缓存。
	readCache *readcache.Cache
}

// PostServiceDeps 是 NewPostService 的依赖清单。
//
// 此前是 15 个位置参数的一行调用，读的人必须在构造函数和调用点之间来回
// 数位置才能确认谁是谁；加一个依赖就得同时改两处并祈祷顺序没错。改成具名
// 结构体后，调用点自己说明字段名，漏填会编译报错，新增依赖也只是加一行。
type PostServiceDeps struct {
	Posts      repository.PostRepository
	Contents   repository.ContentRepository
	TopicReads repository.TopicReadRepository
	Revisions  repository.RevisionRepository
	Users      repository.UserRepository
	SpaceRepo  repository.SpaceRepository

	Ban        BannedService
	Moderation ModerationService
	Settings   SettingService
	Perms      PermissionChecker
	Blocks     UserBlockService
	Badges     BadgeService
	Gate       GateService

	Store db.Store
	KV    kvstore.Store
	Bus   *eventbus.EventBus
}

func NewPostService(d PostServiceDeps) PostService {
	return &postService{
		posts:         d.Posts,
		contents:      d.Contents,
		topicReads:    d.TopicReads,
		revisions:     d.Revisions,
		users:         d.Users,
		spaceRepo:     d.SpaceRepo,
		banSvc:        d.Ban,
		moderationSvc: d.Moderation,
		settingSvc:    d.Settings,
		perms:         d.Perms,
		blockSvc:      d.Blocks,
		badgeSvc:      d.Badges,
		txMgr:         db.NewTransactionManager(d.Store),
		kv:            d.KV,
		bus:           d.Bus,
		trustSvc:      nil,
		gateSvc:       d.Gate,
		readCache:     readcache.New(d.KV, postListCacheTTL),
	}
}

func (s *postService) SetTrustService(svc TrustService) {
	s.trustSvc = svc
}

func (s *postService) List(ctx context.Context, page, pageSize int, tagName, authorID, status string, startTime, endTime *time.Time, spaceName, spaceSlug, spaceID string, includeSub bool, sortBy string) ([]model.PostListItem, int, error) {
	// 只有"游客看公开列表"这一种组合可以共享：登录用户的列表会注入已读状态、
	// 拉黑作者的排除项与等级可见性，跟着人变，进共享缓存就是串数据。
	// 游客的可见性恒为 TL0、无已读状态，结果只由下面这串参数决定。
	anonymous := ctx.Value(viewerIDCtxKey{}) == nil
	var cacheKey string
	if anonymous && s.readCache != nil {
		cacheKey = readcache.Key(postListCachePrefix, page, pageSize, tagName, authorID, status,
			startTime, endTime, spaceName, spaceSlug, spaceID, includeSub, sortBy)
		var cached postListPage
		if s.readCache.Get(ctx, "post_list", cacheKey, &cached) {
			return cached.Items, cached.Total, nil
		}
	}

	filter := repository.PostListFilter{
		TagName:   tagName,
		AuthorID:  authorID,
		StartTime: startTime,
		EndTime:   endTime,
		SpaceName: spaceName,
		SpaceSlug: spaceSlug,
		SpaceID:   spaceID,
		SortBy:    sortBy,
	}
	if spaceID != "" && includeSub && s.spaceRepo != nil {
		if ids, err := s.spaceRepo.FindDescendantIDs(ctx, spaceID); err == nil {
			filter.SpaceIDs = make([]string, 0, 1+len(ids))
			filter.SpaceIDs = append(filter.SpaceIDs, spaceID)
			filter.SpaceIDs = append(filter.SpaceIDs, ids...)
			filter.SpaceID = ""
		} else {
			// repo returns error (e.g. DB failure) -> fallback to single space filter
			filter.SpaceIDs = []string{spaceID}
			filter.SpaceID = ""
		}
	}
	if viewerID, _ := ctx.Value(viewerIDCtxKey{}).(string); viewerID != "" {
		filter.ViewerID = viewerID
		if listView, _ := ctx.Value(listViewCtxKey{}).(string); listView != "" {
			filter.ListView = listView
			if s.topicReads != nil {
				if states, err := s.topicReads.ListByUser(ctx, viewerID); err == nil {
					filter.ReadStates = states
				}
			}
		}
	}
	// Exclude authors blocked by the current viewer (if any).
	if s.blockSvc != nil {
		if viewerID, _ := ctx.Value(viewerIDCtxKey{}).(string); viewerID != "" {
			if ids, _, err := s.blockSvc.ListBlockedIDs(ctx, viewerID, 1, 500); err == nil && len(ids) > 0 {
				filter.ExcludeAuthorIDs = ids
			}
		}
	}
	// 暗区软隔离内容不进主推荐流：仅当未按版面精确浏览（spaceID 为空且无 Slug 过滤）时排除。
	// 精确进入暗区版面时保留，软隔离"可访问、可翻案"的前提才成立。
	if spaceID == "" && spaceSlug == "" {
		filter.ExcludeExiled = true
	}
	// Public timeline default: keep closed topics browsable alongside approved ones.
	// 哨兵 repository.StatusFilterAll 表示「不过滤状态」，只对能看非公开内容的调用者
	// 生效；无权限者无论传什么（含哨兵）一律只见 approved/closed。
	canViewNonPublic := s.canViewNonPublic(ctx)
	switch {
	case canViewNonPublic && status == repository.StatusFilterAll:
		// 不过滤：让后台列表能看到 pending / rejected
	case canViewNonPublic && status != "":
		filter.Status = status
	default:
		filter.StatusIn = []string{model.PostStatusApproved, model.PostStatusClosed}
	}
	if err := s.applyVisibility(ctx, &filter); err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	items, total, err := s.posts.ListWithTotal(ctx, filter, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, err
	}
	if cacheKey != "" {
		s.readCache.Put(ctx, cacheKey, postListPage{Items: items, Total: total})
	}
	return items, total, nil
}

// postListPage 是列表的分页快照，单独建类型是为了让缓存里的 JSON 形状
// 与响应信封解耦（改响应字段不必让所有缓存键失效）。
type postListPage struct {
	Items []model.PostListItem `json:"items"`
	Total int                  `json:"total"`
}

const (
	// postListCachePrefix 同时是失效前缀：写路径按前缀整类删除。
	postListCachePrefix = "cache:postlist:"
	// postListCacheTTL 取 10s：论坛列表允许十秒级延迟，换取热点投票/翻页
	// 全部命中缓存；写入路径另有主动失效，实际窗口更短。
	postListCacheTTL = 10 * time.Second
)

// invalidateListCache 在帖子数据变更后清空列表缓存。
// 失败静默：缓存是尽力而为，TTL 兜底。
func (s *postService) invalidateListCache(ctx context.Context) {
	if s.readCache != nil {
		s.readCache.InvalidatePrefix(ctx, postListCachePrefix)
	}
}

func (s *postService) ListByIDs(ctx context.Context, ids []string) ([]model.PostListItem, error) {
	items, err := s.posts.FindByIDsDetails(ctx, ids)
	if err != nil {
		return nil, err
	}
	if s.canViewNonPublic(ctx) {
		return items, nil
	}
	idx, err := s.visibilityIndex(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	viewerID, level, _ := s.viewerVisibility(ctx)
	visible := make([]model.PostListItem, 0, len(items))
	for _, item := range items {
		if isPublicPostStatus(item.Status) && visibleToViewer(item, viewerID, level, idx) {
			visible = append(visible, item)
		}
	}
	return visible, nil
}

func (s *postService) GetByID(ctx context.Context, id string) (*model.PostListItem, error) {
	item, err := s.posts.FindByIDWithDetails(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
		}
		return nil, err
	}
	if item == nil || item.ID == "" {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
	}
	if !s.canViewPost(ctx, item.Status, item.AuthorID) {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
	}
	if err := s.ensurePostVisible(ctx, *item); err != nil {
		return nil, err
	}
	s.incrViewCount(ctx, id)
	s.recordBrowse(ctx, id)
	return item, nil
}

// recordBrowse 是各「帖子详情」出口共用的被动落库钩子。
// 可见性在调用方已经判过（403/404 都到不了这里），所以直接写仓储，不再做
// RecordView 那次 FindByID。匿名无归属主体，跳过；写失败只降级成「这条没记上」，
// 不能让一次读详情的请求因为读状态表抖动而 500。
func (s *postService) recordBrowse(ctx context.Context, id string) {
	viewerID, _ := ctx.Value(viewerIDCtxKey{}).(string)
	if viewerID == "" || s.topicReads == nil {
		return
	}
	if err := s.topicReads.RecordView(ctx, viewerID, id, "", time.Now()); err != nil {
		logrus.Warnf("post: record browse %s/%s failed: %v", viewerID, id, err)
	}
}

func (s *postService) GetDetail(ctx context.Context, id string) (*model.PostDetail, error) {
	detail, err := s.posts.FindDetailByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
		}
		return nil, err
	}
	if detail == nil || detail.ID == "" {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
	}
	if !s.canViewPost(ctx, detail.Status, detail.AuthorID) {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
	}
	if err := s.ensurePostVisible(ctx, detail.PostListItem); err != nil {
		return nil, err
	}
	s.incrViewCount(ctx, id)
	s.recordBrowse(ctx, id)
	return detail, nil
}

// CheckViewablePost 供从属内容出口（评论/回复列表）复用帖子的可见性判定。
// 与详情保持同一语义：非公开状态 404（作者例外），等级不足 403 并带所需/当前等级。
func (s *postService) CheckViewablePost(ctx context.Context, post *model.Post) error {
	if post == nil || post.ID == "" {
		return pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
	}
	if !s.canViewPost(ctx, post.Status, post.AuthorID) {
		return pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
	}
	return s.ensurePostVisible(ctx, model.PostListItem{
		AuthorID: post.AuthorID,
		SpaceID:  post.SpaceID,
		MinLevel: post.MinLevel,
	})
}

// ensurePostVisible 对直连详情做等级门槛判定：不足时返回 403 且带所需/当前
// 等级（区别于封禁的 404 —— 受限内容是已知存在、明确提示门槛的产品行为）。
func (s *postService) ensurePostVisible(ctx context.Context, item model.PostListItem) error {
	if s.canViewNonPublic(ctx) {
		return nil
	}
	idx, err := s.visibilityIndex(ctx)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	viewerID, level, _ := s.viewerVisibility(ctx)
	if visibleToViewer(item, viewerID, level, idx) {
		return nil
	}
	need := idx.PostRequiredLevel(item.SpaceID, item.MinLevel)
	return pkg.NewAppError(pkg.FORBIDDEN, fmt.Sprintf("需要等级 %d，当前等级 %d", need, level))
}

// incrViewCount increments view count with retry to reduce loss on transient KV failure.
func (s *postService) incrViewCount(ctx context.Context, id string) {
	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		if _, err := s.kv.IncrBy(ctx, "vc:"+id, 1); err == nil {
			return
		} else if i < maxRetries-1 {
			logrus.Warnf("post: view count incr retry %d/%d for %s: %v", i+1, maxRetries, id, err)
		} else {
			logrus.Warnf("post: view count incr failed after %d retries for %s: %v", maxRetries, id, err)
		}
	}
}

// legacyPostApproval 报告旧的布尔开关 post_approval=true 是否仍然生效。
//
// 它只在 moderation_mode 还没有落库时生效：存量站点里 post_approval=true 的语义
// 保持不变，但管理员只要在后台选过审核模式，旧开关就被忽略——否则一条历史配置会
// 把站点永久锁在「全部人工」，而新界面里根本没有能关掉它的开关。
func (s *postService) legacyPostApproval(ctx context.Context) bool {
	if s.settingSvc == nil {
		return false
	}
	if raw, err := s.settingSvc.Get(ctx, model.SettingModerationMode); err == nil && strings.TrimSpace(raw) != "" {
		return false
	}
	return pkg.GetSettingOrDefault(ctx, s.settingSvc, "post_approval", "POST_APPROVAL", "false") == "true"
}

func (s *postService) Create(ctx context.Context, authorID, title, body, spaceID string) (*model.Post, error) {
	// Optional email verification gate
	if s.settingSvc != nil && s.users != nil {
		required := pkg.GetSettingOrDefault(ctx, s.settingSvc, "email_verification_required", "EMAIL_VERIFICATION_REQUIRED", "false")
		if required == "true" {
			if u, err := s.users.FindByID(ctx, authorID); err == nil && u != nil && !u.EmailVerified {
				return nil, pkg.NewAppError(pkg.FORBIDDEN, "请先验证邮箱后再发帖")
			}
		}
	}
	// 验证版面存在
	if spaceID != "" {
		if _, err := s.spaceRepo.FindByID(ctx, spaceID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, pkg.NewAppError(pkg.NOT_FOUND, "版面不存在")
			}
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	// 版面禁言检查
	if spaceID != "" && s.banSvc != nil {
		if s.banSvc.IsBannedInSpace(ctx, authorID, spaceID) {
			return nil, pkg.NewAppError(pkg.USER_BANNED, "您在该版面已被禁言，无法发帖")
		}
	}
	// NGA 版面级发帖权限（层级继承）
	if spaceID != "" && s.perms != nil && s.users != nil {
		if u, err := s.users.FindByID(ctx, authorID); err == nil && u != nil {
			if !s.perms.HasPermissionInSpace(u.Role, spaceID, model.PermPostCreate) {
				return nil, pkg.NewAppError(pkg.FORBIDDEN, "无权限在此版面发帖")
			}
		}
	}
	// 暗区软隔离：被 exile 通过的用户，新发帖一律路由到暗区（design D3）。
	// 只改变落库的目标版面，不拦截发帖、不影响登录——软隔离不等于封号；
	// 翻案（motion:manage overturn）清空 exiled_until 后此路由自动失效。
	routedSpaceID, exiledRouting := s.routeExiledAuthor(ctx, authorID, spaceID)
	spaceID = routedSpaceID

	post, content := model.NewPost(authorID, title, body, spaceID)
	// 由软隔离路由落到暗区的帖子同样打 exiled 标记：主推荐流据此过滤，
	// 否则"路由到暗区"只是换了个版面，内容仍会在时间线上出现。
	if exiledRouting {
		post.Exiled = true
	}
	post.Slug = SlugifyTitle(title)
	if post.Slug != "" && post.ID != "" {
		// disambiguate by suffix of id
		suffix := post.ID
		if len(suffix) > 6 {
			suffix = suffix[len(suffix)-6:]
		}
		// always append short id suffix to reduce collisions
		post.Slug = post.Slug + "-" + strings.ToLower(suffix)
	}

	// 审核模式：off 不审核 / async 发布后异步审核 / external 发布前外部审核 /
	// manual 全部人工。旧布尔开关 post_approval=true 只在 moderation_mode 尚未
	// 落库时等价于「全部人工」（见 legacyPostApproval）。
	mode := model.DefaultModerationMode
	if s.moderationSvc != nil {
		mode = s.moderationSvc.Mode(ctx)
	}
	if s.legacyPostApproval(ctx) {
		mode = model.ModerationModeManual
	}

	// 需要异步复查的帖子：async 恒为真；external 在外部服务不可用时降级为真。
	asyncCheck := mode == model.ModerationModeAsync
	externalAction := model.ModerationAction("")

	// 门控规则引擎：能力域 post_create / external_link。可见性不在这里，
	// 只由版面策略与帖子覆盖决定（见 visibility.go）。
	gateReview := false
	if s.gateSvc != nil && s.users != nil {
		if u, err := s.users.FindByID(ctx, authorID); err == nil && u != nil {
			in := gateInputForUser(u, spaceID, title+"\n"+body)
			// 含外链时才进入外链域：否则域默认动作（deny）会误伤纯文本。
			if in.ContainsExternalLink {
				if d, gerr := s.gateSvc.Evaluate(ctx, model.GateDomainExternalLink, in); gerr == nil && d.Action == model.GateActionDeny {
					return nil, pkg.NewAppError(pkg.INVALID_PARAMS, gateDenyMessage(d, "当前等级不允许发布外链"))
				}
			}
			if d, gerr := s.gateSvc.Evaluate(ctx, model.GateDomainPostCreate, in); gerr == nil {
				switch d.Action {
				case model.GateActionDeny:
					return nil, pkg.NewAppError(pkg.FORBIDDEN, gateDenyMessage(d, "当前条件不允许发帖"))
				case model.GateActionRequireReview:
					// off 是字面意义的「不审核」，此时忽略转待审动作。
					if mode != model.ModerationModeOff {
						gateReview = true
					}
				case model.GateActionRateLimit:
					if err := s.gateSvc.EnforceRateLimit(ctx, d, in); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	if gateReview {
		post.Status = model.PostStatusPending
	}

	switch mode {
	case model.ModerationModeManual:
		post.Status = model.PostStatusPending
	case model.ModerationModeExternal:
		// 发布前同步问外部自动化审核服务；不可用/超时则退回异步内部规则，
		// 不因为第三方抖动卡住发帖。
		action, err := s.moderationSvc.CheckExternal(ctx, "post", post.ID, title, body, model.ModerationScenePost)
		if err != nil {
			logrus.Warnf("post: external moderation unavailable, fallback to async: %v", err)
			asyncCheck = true
		} else {
			externalAction = action
			if action != model.ModerationActionAllow {
				post.Status = model.PostStatusPending
			}
		}
	case model.ModerationModeOff:
		// 不审核：保持 NewPost 的 approved，且不写任何审核记录。
	}

	if err := s.posts.CreateWithContent(ctx, post, content); err != nil {
		return nil, err
	}

	senderName := ""
	role := "user"
	if s.users != nil {
		if u, err := s.users.FindByID(ctx, authorID); err == nil && u != nil {
			senderName = displayNameOf(u)
			if u.Role != "" {
				role = u.Role
			}
		}
	}

	if s.moderationSvc != nil && mode != model.ModerationModeOff {
		switch {
		case externalAction != "" && externalAction != model.ModerationActionAllow:
			// 外部审核判了 flag/block/replace：内容已落库为待审，留痕后交人工。
			if err := s.moderationSvc.RecordPendingFrom(ctx, "post", post.ID, model.ModerationScenePost, externalAction, body, "external"); err != nil {
				logrus.Warnf("post: record external moderation for %s: %v", post.ID, err)
			}
			if externalAction == model.ModerationActionBlock {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "含违规内容")
			}
		case post.Status == model.PostStatusPending:
			// 待审必留痕：manual / TL0 强制待审的帖子必须进队列，否则
			// Approve(id) 只接受审核记录 id → 无路可审。
			strategy := "manual"
			if gateReview {
				strategy = "gate"
			} else if mode != model.ModerationModeManual {
				strategy = "tl0"
			}
			if err := s.moderationSvc.RecordPendingFrom(ctx, "post", post.ID, model.ModerationScenePost, model.ModerationActionFlag, body, strategy); err != nil {
				logrus.Warnf("post: record pending moderation for %s: %v", post.ID, err)
			}
		}
	}

	s.recomputeTrust(ctx, authorID)
	if s.badgeSvc != nil {
		s.badgeSvc.Evaluate(ctx, authorID, "post")
	}
	if s.bus != nil {
		payload := map[string]any{"post_id": post.ID, "author_id": authorID, "title": title}
		if asyncCheck {
			// 只有显式要求异步复查时才让审核消费者接手；人工/免审不出队。
			payload["moderation_async"] = "true"
			payload["author_role"] = role
		}
		s.bus.Publish(eventbus.Event{Type: eventbus.EventPostCreated, Ctx: ctx, Payload: payload})
	}

	// @提及通知（标题 + 正文）
	publishUserMentions(s.bus, s.users, ctx, authorID, post.ID, title+"\n"+body, senderName, map[string]struct{}{})

	s.invalidateListCache(ctx)
	return post, nil
}

func (s *postService) Update(ctx context.Context, id, title, body, actorUUID, actorRole string) (*model.Post, error) {
	var result *model.Post
	err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		post, err := s.posts.FindByID(txCtx, id)
		if err != nil {
			return err
		}
		// R-06b: 作者 OR 有权限
		isAuthor := post.AuthorID == actorUUID
		hasPerm := s.perms != nil && s.perms.HasPermissionInSpace(actorRole, post.SpaceID, model.PermPostUpdate)
		if !isAuthor && !hasPerm {
			return pkg.NewAppError(pkg.FORBIDDEN, "无权编辑该帖子")
		}
		content, err := s.contents.FindByID(txCtx, post.ContentID)
		if err != nil {
			return err
		}
		if s.revisions != nil {
			_ = s.revisions.Create(txCtx, &model.Revision{
				TargetType: "post",
				TargetID:   post.ID,
				Title:      post.Title,
				Body:       content.Body,
				AuthorID:   actorUUID,
				CreatedAt:  time.Now(),
			})
		}
		post.Title = title
		post.ContentDigest = model.DigestBody(body, 200)
		post.UpdatedAt = time.Now()
		content.Body = body
		content.UpdatedAt = time.Now()

		if err := s.contents.Update(txCtx, content); err != nil {
			return err
		}
		if err := s.posts.Update(txCtx, post); err != nil {
			return err
		}
		result = post
		return nil
	})
	if err == nil {
		s.invalidateListCache(ctx)
	}
	return result, err
}

func (s *postService) Delete(ctx context.Context, id, actorUUID, actorRole string) error {
	authorID := ""
	err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		post, err := s.posts.FindByID(txCtx, id)
		if err != nil {
			return err
		}
		// R-06b: 作者 OR 有权限
		isAuthor := post.AuthorID == actorUUID
		hasPerm := s.perms != nil && s.perms.HasPermissionInSpace(actorRole, post.SpaceID, model.PermPostDelete)
		if !isAuthor && !hasPerm {
			return pkg.NewAppError(pkg.FORBIDDEN, "无权删除该帖子")
		}
		authorID = post.AuthorID
		return s.posts.Delete(txCtx, id)
	})
	if err != nil {
		return err
	}
	// 删帖自动回收 XP：行为聚合的输入少了一条，必须重算。
	s.recomputeTrust(ctx, authorID)
	s.invalidateListCache(ctx)
	return nil
}

func (s *postService) Search(ctx context.Context, keyword string, page, pageSize int) ([]model.PostListItem, int, error) {
	filter := repository.PostListFilter{}
	if viewerID, _ := ctx.Value(viewerIDCtxKey{}).(string); viewerID != "" {
		filter.ViewerID = viewerID
	}
	if s.blockSvc != nil {
		if viewerID, _ := ctx.Value(viewerIDCtxKey{}).(string); viewerID != "" {
			if ids, _, err := s.blockSvc.ListBlockedIDs(ctx, viewerID, 1, 500); err == nil && len(ids) > 0 {
				filter.ExcludeAuthorIDs = ids
			}
		}
	}
	if err := s.applyVisibility(ctx, &filter); err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	items, total, err := s.posts.SearchWithTotalFilter(ctx, keyword, filter, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *postService) ListTrash(ctx context.Context, page, pageSize int) ([]model.PostListItem, int, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	return s.posts.ListTrash(ctx, (page-1)*pageSize, pageSize)
}

// SitemapEntries bypasses List() on purpose: List() clamps pageSize to
// pkg.PageMaxSize (100) and applies viewer/block filters that a crawler never
// has. The sitemap needs the whole public corpus in large pages, so it calls the
// repository's dedicated projection directly.
func (s *postService) SitemapEntries(ctx context.Context, page, pageSize int) ([]repository.SitemapEntry, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > model.SitemapMaxPageSize {
		pageSize = model.SitemapMaxPageSize
	}
	idx, err := s.visibilityIndex(ctx)
	if err != nil {
		return nil, err
	}
	// 爬虫面永远是匿名访问：游客 = TL0，不登录。
	publicSpaces := idx.VisibleSpaceIDs(0, false)
	return s.posts.ListSitemapEntries(ctx, (page-1)*pageSize, pageSize, publicSpaces)
}

func (s *postService) CountSitemapEntries(ctx context.Context) (int, error) {
	idx, err := s.visibilityIndex(ctx)
	if err != nil {
		return 0, err
	}
	return s.posts.CountSitemapEntries(ctx, idx.VisibleSpaceIDs(0, false))
}

func (s *postService) RestoreTrash(ctx context.Context, id string) error {
	if id == "" {
		return pkg.NewAppError(pkg.NOT_FOUND, "帖子 ID 不能为空")
	}
	return s.posts.Restore(ctx, id)
}

func (s *postService) Suggestions(ctx context.Context, postID string, limit int) ([]model.PostListItem, error) {
	if limit <= 0 {
		limit = 5
	}
	post, err := s.GetByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{post.ID: {}}
	items := []model.PostListItem{}
	appendUnique := func(candidates []model.PostListItem) {
		for _, item := range candidates {
			if _, ok := seen[item.ID]; ok {
				continue
			}
			seen[item.ID] = struct{}{}
			items = append(items, item)
			if len(items) >= limit {
				return
			}
		}
	}
	if post.SpaceName != "" {
		sameSpace, _, listErr := s.List(ctx, 1, limit+1, "", "", "", nil, nil, post.SpaceName, "", post.SpaceID, false, "active")
		if listErr == nil {
			appendUnique(sameSpace)
		}
	}
	if len(items) < limit {
		hot, _, listErr := s.List(ctx, 1, limit+1, "", "", "", nil, nil, "", "", "", false, "hot")
		if listErr == nil {
			appendUnique(hot)
		}
	}
	if items == nil {
		items = []model.PostListItem{}
	}
	return items, nil
}

func (s *postService) TogglePin(ctx context.Context, id, actorRole string) (*model.Post, error) {
	// 权限检查仍需读取帖子
	post, err := s.posts.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if s.perms == nil || !s.perms.HasPermissionInSpace(actorRole, post.SpaceID, model.PermPostPin) {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "无权置顶该帖子")
	}
	// 使用原子 SQL 翻转，避免 read-modify-write 竞态
	return s.posts.TogglePinned(ctx, id)
}

func (s *postService) ToggleLock(ctx context.Context, id, actorRole string) (*model.Post, error) {
	post, err := s.posts.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if s.perms == nil || !s.perms.HasPermissionInSpace(actorRole, post.SpaceID, model.PermPostLock) {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "无权锁定该帖子")
	}
	return s.posts.ToggleLocked(ctx, id)
}

func (s *postService) SetClosed(ctx context.Context, id string, closed bool, actorID, actorRole string) (*model.Post, error) {
	post, err := s.posts.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	isAuthor := post.AuthorID == actorID
	hasPerm := s.perms != nil && s.perms.HasPermissionInSpace(actorRole, post.SpaceID, model.PermPostLock)
	if !isAuthor && !hasPerm {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "无权关闭或打开该话题")
	}

	if closed {
		switch post.Status {
		case model.PostStatusClosed:
			return post, nil
		case model.PostStatusApproved:
			post.Status = model.PostStatusClosed
		default:
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "仅已通过的话题可关闭")
		}
	} else {
		switch post.Status {
		case model.PostStatusApproved:
			return post, nil
		case model.PostStatusClosed:
			post.Status = model.PostStatusApproved
		default:
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "仅已关闭的话题可重新打开")
		}
	}

	post.UpdatedAt = time.Now()
	if err := s.posts.Update(ctx, post); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if s.bus != nil {
		s.bus.Publish(eventbus.Event{Type: eventbus.EventPostClosed, Ctx: ctx, Payload: map[string]any{"post_id": post.ID, "closed": closed, "actor_id": actorID}})
	}
	return post, nil
}

// SetStatus 直接变更帖子状态（后台「通过 / 拒绝 / 打回待审 / 关闭」）。
//
// 与 moderation.Approve/Reject 的区别：那条路只接受审核记录 id，而历史上被
// 静默置为 pending 的存量帖没有任何审核记录，只能靠本入口救回。权限由网关
// path→permission 映射（report:manage）把守，service 层不再重复判权。
func (s *postService) SetStatus(ctx context.Context, id, status string) error {
	if !isValidPostStatus(status) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "status 必须为 approved、pending、rejected 或 closed")
	}
	post, err := s.posts.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if post.Status == status {
		return nil
	}
	post.Status = status
	post.UpdatedAt = time.Now()
	if err := s.posts.Update(ctx, post); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	// 只有 approved / closed 计 XP：状态变更会改变发帖指标的计数。
	s.recomputeTrust(ctx, post.AuthorID)
	s.invalidateListCache(ctx)
	return nil
}

// SetFeatured 设置 / 取消精华（写 posts.featured_at）。精华是 featured_post_count
// 指标的唯一来源，变更后立即重算作者 XP。
func (s *postService) SetFeatured(ctx context.Context, id string, featured bool) (*model.Post, error) {
	post, err := s.posts.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if featured {
		now := time.Now()
		post.FeaturedAt = &now
	} else {
		post.FeaturedAt = nil
	}
	post.UpdatedAt = time.Now()
	if err := s.posts.Update(ctx, post); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.recomputeTrust(ctx, post.AuthorID)
	return post, nil
}

// recomputeTrust 调用唯一写入点刷新 XP / 等级；失败只记日志，不影响主流程。
func (s *postService) recomputeTrust(ctx context.Context, userID string) {
	if s.trustSvc == nil || userID == "" {
		return
	}
	if _, err := s.trustSvc.Recompute(ctx, userID); err != nil {
		logrus.Warnf("trust: recompute %s: %v", userID, err)
	}
}

// isValidPostStatus reports whether status is one of the four post states the
// status-change endpoint accepts.
func isValidPostStatus(status string) bool {
	switch status {
	case model.PostStatusApproved, model.PostStatusPending, model.PostStatusRejected, model.PostStatusClosed:
		return true
	default:
		return false
	}
}

func (s *postService) SetPin(ctx context.Context, id, actorRole string, req SetPinRequest) (*model.Post, error) {
	post, err := s.posts.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if s.perms == nil || !s.perms.HasPermissionInSpace(actorRole, post.SpaceID, model.PermPostPin) {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "无权置顶该帖子")
	}
	now := time.Now()
	var expireAt *time.Time
	if req.PinnedExpireAt != nil {
		t := time.Unix(*req.PinnedExpireAt, 0)
		expireAt = &t
	}
	return s.posts.SetPin(ctx, id, repository.SetPinInput{
		Pinned: true, PinOrder: req.PinOrder, PinnedAt: &now, PinnedExpireAt: expireAt,
	})
}

func (s *postService) Unpin(ctx context.Context, id, actorRole string) (*model.Post, error) {
	post, err := s.posts.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if s.perms == nil || !s.perms.HasPermissionInSpace(actorRole, post.SpaceID, model.PermPostPin) {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "无权取消置顶该帖子")
	}
	return s.posts.Unpin(ctx, id)
}

func (s *postService) ListPinned(ctx context.Context, spaceID string, page, pageSize int) ([]model.PostListItem, int, error) {
	offset := (page - 1) * pageSize
	vis := repository.PostListFilter{}
	if err := s.applyVisibility(ctx, &vis); err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	items, total, err := s.posts.ListPinnedBySpace(ctx, spaceID, offset, pageSize, vis)
	if err != nil {
		return nil, 0, err
	}
	return items, int(total), nil
}

func (s *postService) FlushViewCounts(ctx context.Context) error {
	entries, err := s.kv.PrefixScan(ctx, "vc:")
	if err != nil {
		return err
	}
	var flushed []string
	for key, val := range entries {
		id := key[len("vc:"):]
		delta, parseErr := strconv.ParseInt(val, 10, 64)
		if parseErr != nil || delta <= 0 {
			continue
		}
		if incErr := s.posts.IncrementViewCountBy(ctx, id, delta); incErr != nil {
			logrus.Warnf("FlushViewCounts: failed to increment view count for post %s: %v", id, incErr)
		} else {
			flushed = append(flushed, key)
		}
	}
	for _, key := range flushed {
		s.kv.Delete(ctx, key)
	}
	return nil
}

func (s *postService) StartViewCountFlusher(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		// 从配置读取刷新间隔，默认 60s
		intervalSec := pkg.GetSettingInt(ctx, s.settingSvc, "view_count_flush_interval", "", 60)
		ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.FlushViewCounts(ctx); err != nil {
					logrus.Warnf("StartViewCountFlusher: flush error: %v", err)
				}
			}
		}
	}()
	return cancel
}

func (s *postService) StartPinExpiryChecker(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := s.posts.ExpirePins(context.Background())
				if err != nil {
					logrus.Warnf("pin expiry sweep failed: %v", err)
				} else if n > 0 {
					logrus.Infof("auto-unpinned %d expired posts", n)
				}
			}
		}
	}()
	return cancel
}

func (s *postService) MarkRead(ctx context.Context, userID, postID string) error {
	if _, err := s.posts.FindByID(ctx, postID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
		}
		return err
	}
	// 不带游标的「标记已读」不再 fallback 写主楼 contentID：那不代表任何阅读进度，
	// 反而会把已有的评论级续读位置抹回帖子开头。留空即「保持原游标」。
	return s.MarkReadToContent(ctx, userID, postID, "")
}

func (s *postService) MarkReadToContent(ctx context.Context, userID, postID, lastContentID string) error {
	if userID == "" || postID == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "参数不完整")
	}
	if s.topicReads == nil {
		return nil
	}
	if err := s.checkCursorForPost(ctx, postID, lastContentID); err != nil {
		return err
	}
	return s.topicReads.Upsert(ctx, userID, postID, lastContentID, time.Now())
}

// checkCursorForPost 校验续读游标指向的评论确实属于这篇帖子。
// 帖子↔评论的唯一关联是 content.RootID == post.ContentID（与 content.go 的判定同源）。
// 少了这一步，任何人拿别人帖子里的一条评论 id 就能盲写自己的读状态，
// 顺便探测那条评论的存在性——MarkReadToContent 历史上只校验「评论是否存在」。
func (s *postService) checkCursorForPost(ctx context.Context, postID, contentID string) error {
	if contentID == "" || s.contents == nil {
		return nil
	}
	rootID := ""
	if s.posts != nil {
		post, err := s.posts.FindByID(ctx, postID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
			}
			return err
		}
		if post == nil {
			return pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
		}
		rootID = post.ContentID
	}
	c, err := s.contents.FindByID(ctx, contentID)
	if err != nil || c == nil || (rootID != "" && c.RootID != rootID) {
		return pkg.NewAppError(pkg.NOT_FOUND, "评论不存在")
	}
	return nil
}

// RecordView 记一次浏览：只写 last_viewed_at 与（给了值时的）续读游标，
// MUST NOT 写 last_read_at——看过 ≠ 读完，写了就会把「有新回复」角标与
// 「没读过」筛选一起改坏。由详情出口自动调用，不依赖前端。
//
// 访问者身份一律取自 ctx（WithViewerID），不从参数收：这四个出口写的都是
// 「谁读过什么」，参数化的 userID 早晚会成为越权写口的入口。
func (s *postService) RecordView(ctx context.Context, postID, lastContentID string) error {
	userID, _ := ctx.Value(viewerIDCtxKey{}).(string)
	if userID == "" {
		// 匿名没有归属主体：静默跳过，不是错误（详情是匿名可读的）。
		return nil
	}
	if postID == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "参数不完整")
	}
	if s.topicReads == nil {
		return nil
	}
	post, err := s.posts.FindByID(ctx, postID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
		}
		return err
	}
	if post == nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
	}
	// 与详情同一门槛：状态不可见按 404（作者/审核者例外），等级不足按 403。
	// 不可见帖不留记录，否则历史接口会变成存在性探测器。
	if !s.canViewPost(ctx, post.Status, post.AuthorID) {
		return pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
	}
	if err := s.ensurePostVisible(ctx, model.PostListItem{
		AuthorID: post.AuthorID,
		SpaceID:  post.SpaceID,
		MinLevel: post.MinLevel,
	}); err != nil {
		return err
	}
	if err := s.checkCursorForPost(ctx, postID, lastContentID); err != nil {
		return err
	}
	return s.topicReads.RecordView(ctx, userID, postID, lastContentID, time.Now())
}

// viewerOrFail 取出 ctx 里的访问者，缺失时给 401（私有出口共用）。
func (s *postService) viewerOrFail(ctx context.Context) (string, error) {
	userID, _ := ctx.Value(viewerIDCtxKey{}).(string)
	if userID == "" {
		return "", pkg.NewAppError(pkg.TOKEN_NOT_EXIST, "请先登录")
	}
	return userID, nil
}

// ListHistory 是「我看过哪些帖、各读到哪条评论」的出口。
// 可见性与状态过滤全部交给 SQL（applyVisibility + StatusIn），
// 在 Go 里筛会让 total 说谎并泄露受限帖的存在性。
func (s *postService) ListHistory(ctx context.Context, page, pageSize int) ([]model.PostListItem, int, error) {
	userID, err := s.viewerOrFail(ctx)
	if err != nil {
		return nil, 0, err
	}
	if s.posts == nil || s.topicReads == nil {
		return []model.PostListItem{}, 0, nil
	}
	filter := repository.PostListFilter{}
	// 与 List 同规则：无审核权限者只见 approved/closed（待审/拒稿不进历史）。
	if !s.canViewNonPublic(ctx) {
		filter.StatusIn = []string{model.PostStatusApproved, model.PostStatusClosed}
	}
	if err := s.applyVisibility(ctx, &filter); err != nil {
		return nil, 0, err
	}
	items, total, err := s.posts.ListReadByUser(ctx, userID, filter, pkg.Offset(page, pageSize), pageSize)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// DeleteHistory 删除访问者自己的一条浏览记录；不存在也返回成功（幂等）。
func (s *postService) DeleteHistory(ctx context.Context, postID string) error {
	userID, err := s.viewerOrFail(ctx)
	if err != nil {
		return err
	}
	if postID == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "参数不完整")
	}
	if s.topicReads == nil {
		return nil
	}
	return s.topicReads.DeleteByUserAndPost(ctx, userID, postID)
}

// ClearHistory 清空访问者自己的全部浏览记录。作用域只可能是 ctx 里的这个人。
func (s *postService) ClearHistory(ctx context.Context) error {
	userID, err := s.viewerOrFail(ctx)
	if err != nil {
		return err
	}
	if s.topicReads == nil {
		return nil
	}
	return s.topicReads.DeleteByUser(ctx, userID)
}

// EnrichWithReadState fills has_new / last_read_at / last_content_id.
// Phase-2 rule: no prior read mark => has_new=false (only tracked topics show new replies).
// 浏览记录改为被动落库后，「有行」不再等于「标过已读」：读标记为 NULL 的行
// 必须继续视同从未读过，否则一次点开就会把全站角标熄灭。游标则要照常回传。
func (s *postService) EnrichWithReadState(ctx context.Context, userID string, items []model.PostListItem) {
	if userID == "" || s.topicReads == nil || len(items) == 0 {
		return
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	states, err := s.topicReads.ListByUserAndPosts(ctx, userID, ids)
	if err != nil {
		logrus.Warnf("post: enrich read state failed for user %s: %v", userID, err)
		return
	}
	byPost := make(map[string]model.TopicReadState, len(states))
	for _, st := range states {
		byPost[st.PostID] = st
	}
	for i := range items {
		st, ok := byPost[items[i].ID]
		if !ok {
			items[i].HasNew = false
			items[i].LastReadAt = nil
			items[i].LastContentID = ""
			continue
		}
		// 续读游标与读标记互相独立：只点开过的帖子也要能回到上次的位置。
		items[i].LastContentID = st.LastContentID
		if st.LastReadAt == nil {
			items[i].LastReadAt = nil
			items[i].HasNew = false
			continue
		}
		items[i].LastReadAt = st.LastReadAt
		items[i].HasNew = items[i].LastActiveAt.After(*st.LastReadAt)
		if s.contents != nil {
			if count, err := s.contents.CountUnreadByRoot(ctx, items[i].ContentID, *st.LastReadAt); err == nil {
				items[i].UnreadContentCount = count
			}
		}
	}
}
