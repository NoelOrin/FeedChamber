import { motionService } from '@feedchamber/api-client';
import { queryOptions } from '@tanstack/react-query';

import { qk } from '~/lib/query/keys';

export type MotionListParams = {
	status?: string;
	target_type?: string;
	page?: number;
	pageSize?: number;
};

export function motionListOptions(params: MotionListParams = {}) {
	return queryOptions({
		queryKey: qk.motions.list(params as Record<string, unknown>),
		queryFn: () => motionService.list(params),
	});
}

export const MOTION_TYPE_LABEL: Record<string, string> = {
	fold: '折叠',
	watch: '观察',
	escalate: '移交版主',
	exile: '放逐暗区',
	reverse: '翻案',
};

export const MOTION_STATUS_LABEL: Record<string, string> = {
	pending: '表决中',
	passed: '已通过',
	rejected: '未通过',
	overturned: '已翻案',
};

/* 动议默认开放期 24h；服务端结算在达法定人数时自动触发。 */
export const MOTION_QUORUM_HINT = '默认法定人数 20（用户类动议加倍）';
