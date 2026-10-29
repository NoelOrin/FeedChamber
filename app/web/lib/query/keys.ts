/**
 * 全站 query key 单一来源。页面与组件只从这里取 key，不要在本地拼数组，
 * 否则 invalidateQueries 会漏掉对方写入的缓存。
 */
export const qk = {
	profile: ['profile'] as const,
	settings: ['settings', 'public'] as const,
	/** 站点自定义层（导航 / 首页模块 / 生效片段 / 主题覆盖）。 */
	customization: ['customization', 'public'] as const,
	home: ['dashboard', 'home'] as const,
	themePreference: ['theme', 'preference'] as const,

	spaces: {
		all: ['spaces'] as const,
		tree: ['spaces', 'tree'] as const,
		flat: ['spaces', 'flat'] as const,
		list: ['spaces', 'list'] as const,
		detail: (slug: string) => ['spaces', 'detail', slug] as const,
		breadcrumb: (slug: string) => ['spaces', 'breadcrumb', slug] as const,
		subtree: (slug: string) => ['spaces', 'subtree', slug] as const,
	},

	tags: {
		all: ['tags'] as const,
		list: (params?: Record<string, unknown>) => ['tags', 'list', params ?? {}] as const,
	},

	posts: {
		all: ['posts'] as const,
		list: (params: Record<string, unknown>) => ['posts', 'list', params] as const,
		detail: (id: string) => ['posts', 'detail', id] as const,
		byIds: (ids: string[]) => ['posts', 'byIds', ids.join(',')] as const,
		suggestions: (query: string) => ['posts', 'suggestions', query] as const,
		tags: (id: string) => ['posts', 'tags', id] as const,
		related: (id: string) => ['posts', 'related', id] as const,
	},

	contents: {
		all: ['contents'] as const,
		list: (postId: string, params?: Record<string, unknown>) => ['contents', 'list', postId, params ?? {}] as const,
		replies: (contentId: string) => ['contents', 'replies', contentId] as const,
	},

	notifications: {
		all: ['notifications'] as const,
		list: (params?: Record<string, unknown>) => ['notifications', 'list', params ?? {}] as const,
		unread: ['notifications', 'unread'] as const,
	},

	pm: {
		all: ['pm'] as const,
		conversations: (params?: Record<string, unknown>) => ['pm', 'conversations', params ?? {}] as const,
		messages: (conversationId: string, params?: Record<string, unknown>) =>
			['pm', 'messages', conversationId, params ?? {}] as const,
	},

	user: {
		all: ['user'] as const,
		byId: (id: string) => ['user', 'byId', id] as const,
		publicProfile: (id: string) => ['user', 'publicProfile', id] as const,
		activity: (id: string, params?: Record<string, unknown>) => ['user', 'activity', id, params ?? {}] as const,
		search: (query: string) => ['user', 'search', query] as const,
		banStatus: (id: string) => ['user', 'banStatus', id] as const,
	},

	follow: {
		all: ['follow'] as const,
		status: (targetId: string) => ['follow', 'status', targetId] as const,
		followers: (id: string, params?: Record<string, unknown>) => ['follow', 'followers', id, params ?? {}] as const,
		following: (id: string, params?: Record<string, unknown>) => ['follow', 'following', id, params ?? {}] as const,
		counts: (id: string) => ['follow', 'counts', id] as const,
	},

	favorite: {
		all: ['favorite'] as const,
		status: (postId: string) => ['favorite', 'status', postId] as const,
		list: (params?: Record<string, unknown>) => ['favorite', 'list', params ?? {}] as const,
	},

	search: (params: Record<string, unknown>) => ['search', params] as const,
	leaderboard: { list: (params?: Record<string, unknown>) => ['leaderboard', 'list', params ?? {}] as const },
	checkin: {
		status: ['checkin', 'status'] as const,
		history: (month: string) => ['checkin', 'history', month] as const,
	},
	announcements: { published: ['announcements', 'published'] as const },
	badges: {
		all: ['badges'] as const,
		byUser: (id: string) => ['badges', 'byUser', id] as const,
	},
	drafts: { all: ['drafts'] as const, list: ['drafts', 'list'] as const },
	motions: {
		all: ['motions'] as const,
		list: (params?: Record<string, unknown>) => ['motions', 'list', params ?? {}] as const,
		detail: (id: string) => ['motions', 'detail', id] as const,
	},

	/**
	 * 后台各模块的取数 key。
	 *
	 * 后台此前 32 个页面全部手写 useEffect + useState，没有任何缓存层，
	 * 也没有统一的失效入口——改一个标签要手动调 fetchData() 刷整张表。
	 * 统一收在这里之后，写操作只需 invalidate 对应前缀。
	 *
	 * 约定：每个模块一个 `all` 前缀 + 列表/详情两级 key，列表 key 带上参数，
	 * 详情 key 带 id，这样 `qk.admin.tags.all` 能一次失效该模块全部缓存。
	 */
	admin: {
		all: ['admin'] as const,
		dashboard: ['admin', 'dashboard'] as const,
		tags: {
			all: ['admin', 'tags'] as const,
			list: (params?: Record<string, unknown>) => ['admin', 'tags', 'list', params ?? {}] as const,
		},
		announcements: {
			all: ['admin', 'announcements'] as const,
			list: (params?: Record<string, unknown>) => ['admin', 'announcements', 'list', params ?? {}] as const,
			detail: (id: string) => ['admin', 'announcements', 'detail', id] as const,
		},
		users: {
			all: ['admin', 'users'] as const,
			list: (params?: Record<string, unknown>) => ['admin', 'users', 'list', params ?? {}] as const,
			detail: (id: string) => ['admin', 'users', 'detail', id] as const,
			activity: (id: string) => ['admin', 'users', 'activity', id] as const,
		},
		posts: {
			all: ['admin', 'posts'] as const,
			list: (params?: Record<string, unknown>) => ['admin', 'posts', 'list', params ?? {}] as const,
			detail: (id: string) => ['admin', 'posts', 'detail', id] as const,
		},
		comments: {
			all: ['admin', 'comments'] as const,
			list: (params?: Record<string, unknown>) => ['admin', 'comments', 'list', params ?? {}] as const,
			detail: (id: string) => ['admin', 'comments', 'detail', id] as const,
		},
		spaces: {
			all: ['admin', 'spaces'] as const,
			list: (params?: Record<string, unknown>) => ['admin', 'spaces', 'list', params ?? {}] as const,
			rolePermission: (spaceId: string, roleName: string) =>
				['admin', 'spaces', 'rolePermission', spaceId, roleName] as const,
		},
		gates: {
			rules: ['admin', 'gates', 'rules'] as const,
			defaults: ['admin', 'gates', 'defaults'] as const,
			meta: ['admin', 'gates', 'meta'] as const,
		},
		trust: {
			levels: ['admin', 'trust', 'levels'] as const,
			xpRules: ['admin', 'trust', 'xp-rules'] as const,
			streak: ['admin', 'trust', 'streak'] as const,
			userBreakdown: (id: string) => ['admin', 'trust', 'user', id] as const,
			userAdjustments: (id: string) => ['admin', 'trust', 'user', id, 'adjustments'] as const,
		},
		roles: {
			all: ['admin', 'roles'] as const,
			list: ['admin', 'roles', 'list'] as const,
			detail: (id: string) => ['admin', 'roles', 'detail', id] as const,
			permissions: (roleName: string) => ['admin', 'roles', 'permissions', roleName] as const,
		},
		permissions: {
			all: ['admin', 'permissions'] as const,
			list: ['admin', 'permissions', 'list'] as const,
		},
		groups: {
			all: ['admin', 'groups'] as const,
			list: ['admin', 'groups', 'list'] as const,
		},
		banned: {
			all: ['admin', 'banned'] as const,
			list: (params?: Record<string, unknown>) => ['admin', 'banned', 'list', params ?? {}] as const,
		},
		reports: {
			all: ['admin', 'reports'] as const,
			list: (params?: Record<string, unknown>) => ['admin', 'reports', 'list', params ?? {}] as const,
			detail: (id: string) => ['admin', 'reports', 'detail', id] as const,
		},
		invites: {
			all: ['admin', 'invites'] as const,
			list: ['admin', 'invites', 'list'] as const,
		},
		apiKeys: {
			all: ['admin', 'apiKeys'] as const,
			list: ['admin', 'apiKeys', 'list'] as const,
		},
		webhooks: {
			all: ['admin', 'webhooks'] as const,
			list: ['admin', 'webhooks', 'list'] as const,
			deliveries: (id: string) => ['admin', 'webhooks', id, 'deliveries'] as const,
		},
		backups: {
			all: ['admin', 'backups'] as const,
			list: ['admin', 'backups', 'list'] as const,
		},
		oauthClients: {
			all: ['admin', 'oauthClients'] as const,
			list: ['admin', 'oauthClients', 'list'] as const,
		},
		plugins: {
			all: ['admin', 'plugins'] as const,
			list: ['admin', 'plugins', 'list'] as const,
		},
		security: {
			all: ['admin', 'security'] as const,
			events: (params?: Record<string, unknown>) => ['admin', 'security', 'events', params ?? {}] as const,
			history: (params?: Record<string, unknown>) => ['admin', 'security', 'history', params ?? {}] as const,
		},
		sessions: {
			all: ['admin', 'sessions'] as const,
			list: (params?: Record<string, unknown>) => ['admin', 'sessions', 'list', params ?? {}] as const,
		},
		trash: {
			all: ['admin', 'trash'] as const,
			posts: (params?: Record<string, unknown>) => ['admin', 'trash', 'posts', params ?? {}] as const,
			comments: (params?: Record<string, unknown>) => ['admin', 'trash', 'comments', params ?? {}] as const,
		},
		moderation: {
			all: ['admin', 'moderation'] as const,
			pending: (params?: Record<string, unknown>) => ['admin', 'moderation', 'pending', params ?? {}] as const,
			rules: ['admin', 'moderation', 'rules'] as const,
			configs: ['admin', 'moderation', 'configs'] as const,
		},
		auditLogs: {
			all: ['admin', 'auditLogs'] as const,
			list: (params?: Record<string, unknown>) => ['admin', 'auditLogs', 'list', params ?? {}] as const,
		},
		settings: {
			all: ['admin', 'settings'] as const,
			list: ['admin', 'settings', 'list'] as const,
			public: ['admin', 'settings', 'public'] as const,
		},
	},
};
