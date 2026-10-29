import { post } from '../client';

export interface GateCondition {
	field: string;
	op: string;
	value: unknown;
	window_days?: number;
}

export interface GateRule {
	id: string;
	priority: number;
	enabled: boolean;
	domain: string;
	/** JSON 字符串（[]GateCondition），表单层负责解析/序列化 */
	conditions: string;
	action: string;
	/** JSON 字符串（对象） */
	action_params: string;
	note: string;
	updated_by?: string;
	updated_at?: string;
}

export interface GateDomainDefault {
	domain: string;
	default_action: string;
	updated_at?: string;
}

export interface GateDecision {
	domain: string;
	action: string;
	rule_id?: string;
	priority?: number;
	note?: string;
	is_default: boolean;
	action_params?: Record<string, unknown>;
}

export interface GateMeta {
	domains: Record<string, string>;
	fields: Record<string, string>;
	field_ops: Record<string, string[]>;
	actions: string[];
	window_metric: Record<string, string>;
}

export interface GateSimulateInput {
	user_id: string;
	domain: string;
	space_id?: string;
	tags?: string[];
	is_author?: boolean;
	author_level?: number;
	content?: string;
	contains_external_link?: boolean;
}

export const gateService = {
	meta() {
		return post<GateMeta>('/admin/gates/meta');
	},
	listRules() {
		return post<GateRule[]>('/admin/gates/list');
	},
	saveRule(rule: GateRule) {
		return post<null>('/admin/gates/save', rule);
	},
	deleteRule(id: string) {
		return post<null>(`/admin/gates/${id}/delete`);
	},
	listDefaults() {
		return post<GateDomainDefault[]>('/admin/gates/defaults/list');
	},
	saveDefault(domain: string, defaultAction: string) {
		return post<null>('/admin/gates/defaults/save', { domain, default_action: defaultAction });
	},
	simulate(input: GateSimulateInput) {
		return post<GateDecision>('/admin/gates/simulate', input);
	},
};
