import type { User } from '@orinforum/api-client';
import { create } from 'zustand';

import { broadcastAuthEvent } from './sync';

type AuthStatus = 'unknown' | 'anonymous' | 'loading' | 'authenticated' | 'refreshing' | 'expired';

interface AuthState {
	status: AuthStatus;
	user: User | null;
	setLoading: () => void;
	setRefreshing: () => void;
	setAuthenticated: (user: User, broadcast?: boolean) => void;
	setAnonymous: (broadcast?: boolean) => void;
	setExpired: (broadcast?: boolean) => void;
}

export const useAuthStore = create<AuthState>((set) => ({
	status: 'unknown',
	user: null,
	setLoading: () => set({ status: 'loading' }),
	setRefreshing: () => set({ status: 'refreshing' }),
	setAuthenticated: (user, broadcast = false) => {
		set({ status: 'authenticated', user });
		if (broadcast) broadcastAuthEvent('AUTH_LOGIN');
	},
	setAnonymous: (broadcast = false) => {
		set({ status: 'anonymous', user: null });
		if (broadcast) broadcastAuthEvent('AUTH_LOGOUT');
	},
	setExpired: (broadcast = false) => {
		set({ status: 'expired', user: null });
		if (broadcast) broadcastAuthEvent('AUTH_EXPIRED');
	},
}));

/** Imperative façade kept from the Solid implementation so call sites stay cheap to port. */
export const authSession = {
	get status(): AuthStatus {
		return useAuthStore.getState().status;
	},
	get user(): User | null {
		return useAuthStore.getState().user;
	},
	get isAuthenticated(): boolean {
		const state = useAuthStore.getState();
		return state.status === 'authenticated' && state.user !== null;
	},
	setLoading: () => useAuthStore.getState().setLoading(),
	setRefreshing: () => useAuthStore.getState().setRefreshing(),
	setAuthenticated: (user: User, broadcast = false) => useAuthStore.getState().setAuthenticated(user, broadcast),
	setAnonymous: (broadcast = false) => useAuthStore.getState().setAnonymous(broadcast),
	setExpired: (broadcast = false) => useAuthStore.getState().setExpired(broadcast),
};

export function setSessionUser(user: User): void {
	authSession.setAuthenticated(user);
}
