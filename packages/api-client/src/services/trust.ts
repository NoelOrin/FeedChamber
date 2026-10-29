import { get, post, postPaginated } from '../client';

/** 等级定义（后台可配：名称 / XP 门槛 / 保级条件）。 */
export interface TrustLevel {
	level: number;
	key: string;
	name: string;
	min_xp: number;
	window_days: number;
	min_active_days: number;
	min_posts: number;
	min_likes: number;
	/** null = 不限制；0 = 一次成立举报都不允许 */
	max_reports: number | null;
	manual_only: boolean;
	enabled: boolean;
	updated_at?: string;
}

/** 指标 → XP 规则。daily_cap 为 0 表示不做自然日截断。 */
export interface XPRule {
	metric: string;
	label?: string;
	delta: number;
	daily_cap: number;
	enabled: boolean;
	updated_at?: string;
}

/** 连续签到里程碑：恰好达到 days 天时一次性给 bonus。 */
export interface XPStreakBonus {
	days: number;
	bonus: number;
	enabled: boolean;
	updated_at?: string;
}

/** 公开等级配置（/levels）：前端渲染名称与升级进度用。 */
export interface PublicTrustLevel {
	level: number;
	key: string;
	name: string;
	min_xp: number;
}

export interface XPAdjustment {
	id: string;
	user_id: string;
	delta: number;
	reason: string;
	actor_id: string;
	created_at: string;
}

export interface UserTrust {
	user_id: string;
	total_xp: number;
	trust_level: number;
	level_key: string;
	level_name: string;
	level_manual: boolean;
	level_evaluated_at?: string;
	metrics: {
		active_days: number;
		posts: number;
		likes: number;
		upheld_reports: number;
	};
}

export interface XPMetricContribution {
	metric: string;
	label: string;
	count: number;
	delta: number;
	xp: number;
}

export interface XPStreakContribution {
	days: number;
	hits: number;
	bonus: number;
	xp: number;
}

/** 指标分解：各项之和（含人工调整）等于 total_xp。 */
export interface XPBreakdown extends UserTrust {
	xp_metrics: XPMetricContribution[];
	xp_streaks: XPStreakContribution[];
	xp_adjustments: number;
}

export const trustService = {
	/** 公开等级名称表（无需登录；名称/门槛改动后台即生效）。 */
	getPublicLevels() {
		return get<PublicTrustLevel[]>('/levels');
	},

	// 等级定义
	listLevels() {
		return post<TrustLevel[]>('/admin/levels/list');
	},
	saveLevel(level: TrustLevel) {
		return post<null>('/admin/levels/save', level);
	},
	deleteLevel(level: number) {
		return post<null>(`/admin/levels/${level}/delete`);
	},

	// XP 规则
	listXPRules() {
		return post<XPRule[]>('/admin/xp-rules/list');
	},
	saveXPRule(rule: XPRule) {
		return post<null>('/admin/xp-rules/save', rule);
	},

	// 连续签到阶梯
	listStreakBonuses() {
		return post<XPStreakBonus[]>('/admin/xp-rules/streak/list');
	},
	saveStreakBonus(bonus: XPStreakBonus) {
		return post<null>('/admin/xp-rules/streak/save', bonus);
	},
	deleteStreakBonus(days: number) {
		return post<null>(`/admin/xp-rules/streak/${days}/delete`);
	},

	/** 管理员手工设级（manual=true 锁定，false 解除锁定并回到自动求值）。 */
	setUserLevel(userId: string, level: number, manual = true) {
		return post<null>(`/user/${userId}/trust_level/update`, { level, manual });
	},

	// 全量重算
	recomputeAll() {
		return post<{ count: number }>('/admin/trust/recompute');
	},

	// 用户 XP
	getUserXP(userId: string) {
		return get<XPBreakdown>(`/admin/users/${userId}/xp`);
	},
	adjustXP(userId: string, body: { delta: number; reason: string }) {
		return post<XPAdjustment>(`/admin/users/${userId}/xp/adjust`, body);
	},
	listAdjustments(userId: string, params?: { page?: number; page_size?: number }) {
		return postPaginated<XPAdjustment[]>(`/admin/users/${userId}/xp/adjustments/list`, params ?? {});
	},
	deleteAdjustment(id: string) {
		return post<null>(`/admin/xp-adjustments/${id}/delete`);
	},
};
