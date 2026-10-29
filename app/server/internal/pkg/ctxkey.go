package pkg

// 请求上下文键（iris Context.Values）。
//
// 这是一份跨包契约：只有 internal/gateway 在解析凭证后写入，internal/middleware、
// internal/handler 与 internal/service 的少量入口读取。此前两边各自写字符串
// 字面量，键名对不上时不会编译报错，只会表现为"登录后仍提示未登录"这类难查的
// 症状，因此集中成常量。
//
// 值语义：
//   - CtxUserID 装的是 User.ID（ULID）。键名里的 "uuid" 是历史遗留：
//     User.UUID 字段早已废弃（统一用 ULID 主键），但 user_sessions.user_uuid
//     列名与这个键名都还留着。改名涉及迁移，暂未处理。
//   - CtxSession 装的是 *model.UserSession，由会话鉴权路径写入；API Key 与
//     OAuth client 请求没有会话，读它会得到 nil。
//   - CtxAuthMethod 区分凭证来源，取值 session（默认，不写）/ api_key / oauth_client，
//     权限层据此决定用 scopes 还是角色权限表。
const (
	CtxUserID       = "user_uuid"
	CtxUsername     = "username"
	CtxRole         = "user_role"
	CtxSession      = "session"
	CtxClaims       = "claims"
	CtxAuthMethod   = "auth_method"
	CtxAPIKeyScopes = "api_key_scopes"
	CtxClientScopes = "client_scopes"
)

// 凭证来源取值，配合 CtxAuthMethod 使用。
const (
	AuthMethodAPIKey      = "api_key"
	AuthMethodOAuthClient = "oauth_client"
)
