import { post } from '../client';
import type { AuthResponse } from '../types';

export const authService = {
	register(body: { username: string; email: string; password: string; invite_code?: string }) {
		return post<AuthResponse>('/auth/register', body);
	},

	approveRegistration(userId: string) {
		return post<null>(`/admin/users/${userId}/approve_registration`);
	},

	login(body: { login: string; password: string }) {
		return post<AuthResponse>('/auth/login', body);
	},

	logout() {
		return post<null>('/auth/logout');
	},

	refresh() {
		return post<{ access_token: string }>('/auth/refresh');
	},

	changePassword(body: { old_password: string; new_password: string }) {
		return post<null>('/auth/change_password', body);
	},

	firstChangePassword(body: { new_password: string; name?: string }) {
		return post<null>('/auth/first_change_password', body);
	},

	resetPassword(body: { username: string; new_password: string }) {
		return post<null>('/auth/reset_password', body);
	},

	forgotPassword(body: { email: string }) {
		return post<null>('/auth/forgot_password', body);
	},

	resetPasswordByCode(body: { email: string; code: string; new_password: string }) {
		return post<null>('/auth/reset_password_by_code', body);
	},

	sendVerifyEmail() {
		return post<null>('/auth/send_verify_email', {});
	},

	verifyEmail(body: { code: string }) {
		return post<null>('/auth/verify_email', body);
	},
};
