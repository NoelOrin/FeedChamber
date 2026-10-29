import { type AuthResponse, authService, clearAuthTokens, type User, userService } from '@feedchamber/api-client';

import { authSession } from './session';

export async function login(payload: { login: string; password: string }): Promise<User> {
	const response = await authService.login(payload);
	const user = response.user ?? (await userService.getProfile());
	authSession.setAuthenticated(user, true);
	return user;
}

export async function register(payload: {
	username: string;
	email: string;
	password: string;
	invite_code?: string;
}): Promise<AuthResponse> {
	const response = await authService.register(payload);
	const user = response.user ?? (await userService.getProfile());
	authSession.setAuthenticated(user, true);
	return response;
}

export async function logout(): Promise<void> {
	try {
		await authService.logout();
	} finally {
		clearAuthTokens();
		authSession.setAnonymous(true);
	}
}

export async function loadProfile(): Promise<User> {
	authSession.setLoading();
	try {
		const user = await userService.getProfile();
		authSession.setAuthenticated(user);
		return user;
	} catch (error) {
		authSession.setAnonymous();
		throw error;
	}
}

export const authClient = {
	login,
	register,
	logout,
	loadProfile,
	changePassword: (payload: { old_password: string; new_password: string }) => authService.changePassword(payload),
	firstChangePassword: (payload: { new_password: string; name?: string }) => authService.firstChangePassword(payload),
};
