import { get } from '../client';

export type SecurityEvent = {
	id: number;
	user_uuid: string;
	ip: string;
	event_type: string;
	severity: string;
	detail: string;
	action: string;
	handled: boolean;
	created_at: number;
};

export type LoginHistory = {
	id: number;
	user_uuid: string;
	ip: string;
	location: string;
	user_agent: string;
	success: boolean;
	created_at: string;
};

export const securityService = {
	listEvents() {
		return get<SecurityEvent[]>('/security/events');
	},
	listLoginHistory() {
		return get<LoginHistory[]>('/security/login-history');
	},
};
