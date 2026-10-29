package eventbus

// 事件类型常量全集 — 平台化扩展点文档的唯一权威来源。
// 订阅/发布必须引用以下常量，禁止散落字符串字面量。
const (
	EventPostCreated    = "post.created"
	EventPostClosed     = "post.closed"
	EventContentCreated = "content.created"
	EventContentLiked   = "content.liked"
	EventContentReplied = "content.replied"
	EventContentWatched = "content.watched"
	EventUserCreated    = "user.created"
	EventUserRegistered = "user.registered" // 历史别名：等价于 EventUserCreated，Auth 同时双发
	EventUserFollowed   = "user.followed"
	EventUserMentioned  = "user.mentioned"
	EventReportCreated  = "report.created"
	EventModerationDone = "moderation.decided"
	EventMotionCreated  = "motion.created"
	EventMotionResolved = "motion.resolved"
)

// AllEventTypes returns every event type constant, so wiring can seed the
// per-type metric series at startup (metrics.SeedEventbusEvent). Keep in sync
// with the const block above — an event missing here loses its zero-valued
// series, which is exactly what the absent() canary in the alert rules detects.
func AllEventTypes() []string {
	return []string{
		EventPostCreated,
		EventPostClosed,
		EventContentCreated,
		EventContentLiked,
		EventContentReplied,
		EventContentWatched,
		EventUserCreated,
		EventUserRegistered,
		EventUserFollowed,
		EventUserMentioned,
		EventReportCreated,
		EventModerationDone,
		EventMotionCreated,
		EventMotionResolved,
	}
}
