export interface ApiResponse<T = unknown> {
	code: number;
	msg: string;
	data: T | null;
}

export interface PaginatedResponse<T = unknown> {
	code: number;
	msg: string;
	data: {
		items: T;
		total: number;
	} | null;
}

export interface ListParams {
	page?: number;
	page_size?: number;
	start_time?: string;
	end_time?: string;
}

export interface PostListParams extends ListParams {
	tag_name?: string;
	author_id?: string;
	space_id?: string;
	space_name?: string;
	space_slug?: string;
	include_sub?: boolean;
	status?: string;
	sort_by?: 'latest' | 'hot' | 'most_commented' | 'active';
	list_view?: 'new' | 'unread' | 'read' | 'posted';
}

export interface AuthResponse {
	/** Opaque user-session id; also accepted as `Authorization: Bearer <value>`. */
	access_token: string;
	user: User;
	/** When true, the account must change its password before normal use (R-05). */
	must_change_password?: boolean;
}

export interface User {
	id: string;
	username: string;
	display_name: string;
	email: string;
	avatar?: string;
	bio?: string;
	cover_image?: string;
	profile_layout?: string;
	role: string;
	total_xp?: number;
	/** 数值等级（0-4，可扩展）。 */
	trust_level?: number;
	created_at: string;
	updated_at: string;
	email_verified?: boolean;
}

export interface LoginParams {
	login: string;
	password: string;
}

export interface RegisterParams {
	username: string;
	email: string;
	password: string;
	invite_code?: string;
}

export interface Invite {
	id: string;
	code: string;
	creator_id: string;
	max_uses: number;
	used_count: number;
	expires_at?: string | null;
	created_at: string;
}

export interface ChangePasswordParams {
	old_password: string;
	new_password: string;
}

export interface AuthResult {
	code: number;
	msg?: string;
	data?: AuthResponse;
}

export interface Post {
	id: string;
	slug?: string;
	title: string;
	content_id: string;
	content_digest: string;
	author_id: string;
	status: string;
	min_level?: number;
	pinned: boolean;
	featured_at?: string | null;
	locked: boolean;
	last_active_at?: string;
	view_count: number;
	comment_count: number;
	like_count: number;
	favorite_count: number;
	deleted_at?: string;
	created_at: string;
	updated_at: string;
}

export interface PostListItem {
	id: string;
	title: string;
	slug?: string;
	content_id: string;
	content_digest: string;
	author_id: string;
	author_name: string;
	author_avatar: string;
	space_id: string;
	space_name: string;
	status: string;
	min_level?: number;
	pinned: boolean;
	featured_at?: string | null;
	locked: boolean;
	last_active_at?: string;
	view_count: number;
	comment_count: number;
	like_count: number;
	favorite_count: number;
	has_new?: boolean;
	last_read_at?: string | null;
	unread_content_count?: number;
	created_at: string;
	updated_at: string;
}

export interface PostDetail extends PostListItem {
	body: string;
}

export interface Content {
	id: string;
	author_id: string;
	body: string;
	root_id: string;
	parent_id: string;
	like_count: number;
	deleted_at?: string;
	created_at: string;
	updated_at: string;
}

export interface ContentListItem {
	id: string;
	author_id: string;
	author_name: string;
	author_avatar: string;
	body: string;
	root_id: string;
	parent_id: string;
	like_count: number;
	post_id: string;
	post_title: string;
	created_at: string;
	updated_at: string;
}

export interface Tag {
	id: string;
	name: string;
	created_at: string;
}

export interface Space {
	id: string;
	name: string;
	description: string;
	icon: string;
	slug: string;
	sort_order: number;
	parent_id: string;
	depth: number;
	path: string;
	is_active: boolean;
	/** public | login | level（旧字段 auth_required 已由服务端迁移并清零） */
	visibility?: 'public' | 'login' | 'level';
	min_level?: number;
	auth_required?: boolean;
	/** 系统版面（暗区）：不可被普通用户管理 */
	system?: boolean;
	/** 是否进入主推荐流；暗区 = false */
	recommendable?: boolean;
	created_at: string;
	updated_at: string;
}

export interface SpaceNode extends Space {
	children: SpaceNode[];
	level: number;
	has_children: boolean;
	children_count: number;
	descendant_count: number;
}

export interface Notification {
	id: string;
	user_id: string;
	type: string;
	content: string;
	reference_id: string;
	sender_id: string;
	is_read: boolean;
	created_at: string;
}

export interface Upload {
	id: string;
	file_name: string;
	stored_name: string;
	key: string;
	path: string;
	url: string;
	size: number;
	mime_type: string;
	uploader_id: string;
	created_at: string;
}

export interface UploadPresignData {
	upload_id: string;
	url: string;
	fields: Record<string, string>;
	file_url: string;
}

export interface Report {
	id: string;
	reporter_id: string;
	target_type: string;
	target_id: string;
	flag_type?: string;
	reason: string;
	target_title?: string;
	target_body?: string;
	count?: number;
	status: string;
	action?: string;
	handled_by?: string;
	handled_at?: string;
	note?: string;
	created_at: string;
	updated_at: string;
}

// ── 社区自治动议（community motion）─────────────────────────────────────────
// 与 Report 并行：举报=违规上行→管理员；动议=灰区→社区集体裁判。

export type MotionType = 'fold' | 'watch' | 'escalate' | 'exile' | 'reverse';
export type MotionStatus = 'pending' | 'passed' | 'rejected' | 'overturned';
export type MotionTargetType = 'post' | 'comment' | 'user';
export type MotionVoteSide = 'approve' | 'reject';

export interface Motion {
	id: string;
	target_type: MotionTargetType;
	target_id: string;
	type: MotionType;
	reason: string;
	status: MotionStatus;
	initiator_id: string;
	approve_weight: number;
	reject_weight: number;
	vote_count: number;
	abuse_flag: boolean;
	source_motion_id?: string;
	exiled_until?: string;
	resolved_at?: string;
	created_at: string;
	updated_at: string;
}

export interface MotionVote {
	id: string;
	motion_id: string;
	voter_id: string;
	side: MotionVoteSide;
	weight: number;
	created_at: string;
}

export interface MotionListItem {
	id: string;
	target_type: MotionTargetType;
	target_id: string;
	type: MotionType;
	reason: string;
	status: MotionStatus;
	initiator_id: string;
	approve_weight: number;
	reject_weight: number;
	vote_count: number;
	abuse_flag: boolean;
	exiled_until?: string;
	resolved_at?: string;
	created_at: string;
}

export interface Role {
	id: string;
	name: string;
	display_name?: string;
	description?: string;
	is_default?: boolean;
	created_at: string;
}

export interface Permission {
	id: string;
	code: string;
	name: string;
	group?: string;
}

export interface AdminStats {
	total_users: number;
	total_posts: number;
	total_comments: number;
	today_new_users: number;
	today_new_posts: number;
	today_new_comments: number;
}

export interface SettingItem {
	key: string;
	value: string;
	updated_at?: string;
}

export interface BannedUser {
	id: string;
	user_id: string;
	username: string;
	email?: string;
	reason: string;
	banned_by: string;
	banned_at: string;
	expires_at?: string;
	is_permanent: boolean;
}

export interface Announcement {
	id: string;
	title: string;
	content: string;
	status: string;
	is_pinned: boolean;
	display_type: number;
	position: string;
	sort: number;
	expires_at: string;
	author_id: string;
	view_count: number;
	created_at: string;
	updated_at: string;
}

export interface DashboardData {
	stats: AdminStats;
	pending_reports: number;
	recent_users: User[];
	recent_posts: PostListItem[];
}

export interface UserDashboardResponse {
	user: User | null;
	follow_counts: { followers: number; following: number } | null;
	announcements: { items: Announcement[] };
}

export interface AdminUser {
	id: string;
	username: string;
	display_name: string;
	email: string;
	avatar?: string;
	bio?: string;
	role: string;
	trust_level?: number;
	total_xp?: number;
	created_at: string;
	updated_at: string;
}

export interface PaginatedUsers {
	code: number;
	msg?: string;
	data: {
		items: AdminUser[];
		total: number;
	};
}

export interface EntitySearchResult<T> {
	items: T[];
	total: number;
	page: number;
	page_size: number;
}

export interface SearchResults {
	users: EntitySearchResult<User>;
	posts: EntitySearchResult<PostListItem>;
	comments: EntitySearchResult<ContentListItem>;
}

export interface UserActivities {
	user: User;
	posts: PostListItem[];
	posts_total: number;
	comments: ContentListItem[];
	comments_total: number;
	reports_about: Report[];
	reports_total: number;
	reports_by: Report[];
	reports_by_total: number;
}

export interface OAuthProviderConfig {
	enabled: boolean;
	scopes: string[];
	auth_url: string;
	token_url: string;
	user_api_url: string;
	callback_url: string;
}

export interface LinkedOAuthAccount {
	id: string;
	provider: string;
	provider_user_id: string;
	avatar?: string;
	name?: string;
	email?: string;
	linked_at: string;
}

export interface PublicProfile {
	user: PublicUser;
	post_count: number;
	comment_count: number;
	like_count: number;
	is_following?: boolean;
}

export interface PublicUser {
	id: string;
	username: string;
	display_name: string;
	avatar?: string;
	bio?: string;
	cover_image?: string;
	profile_layout?: string;
	role: string;
	trust_level?: number;
	total_xp?: number;
	created_at: string;
}

export interface ActivityResponse {
	year: number;
	entries: Record<string, number>;
}

export interface Conversation {
	id: string;
	created_at: string;
	updated_at: string;
	last_message_at?: string | null;
}

export interface Message {
	id: string;
	conversation_id: string;
	sender_id: string;
	body: string;
	created_at: string;
}

export interface ConversationListItem {
	id: string;
	peer_id: string;
	peer_name: string;
	peer_avatar: string;
	last_message?: string;
	last_message_at?: string | null;
	unread_count: number;
	created_at: string;
	updated_at: string;
}

export interface AuditLog {
	id: string;
	actor_id: string;
	action: string;
	target: string;
	detail?: string;
	created_at: string;
}

export interface Group {
	id: string;
	name: string;
	display_name: string;
	visibility: string;
	is_system: boolean;
	created_at: string;
}

export interface GroupMember {
	group_id: string;
	user_id: string;
	role: string;
	joined_at: string;
}
