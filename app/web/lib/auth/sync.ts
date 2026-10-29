type AuthSyncEvent = 'AUTH_LOGIN' | 'AUTH_LOGOUT' | 'AUTH_REFRESHED' | 'AUTH_EXPIRED' | 'PROFILE_UPDATED';

const CHANNEL_NAME = 'feedchamber-auth';
let channel: BroadcastChannel | null = null;

function getChannel(): BroadcastChannel | null {
	if (typeof window === 'undefined' || typeof BroadcastChannel === 'undefined') return null;
	if (!channel) channel = new BroadcastChannel(CHANNEL_NAME);
	return channel;
}

export function broadcastAuthEvent(type: AuthSyncEvent): void {
	if (typeof window === 'undefined') return;
	getChannel()?.postMessage({ type });
	try {
		localStorage.setItem(CHANNEL_NAME, JSON.stringify({ type, at: Date.now() }));
	} catch {
		// ignore storage fallback failures
	}
}

export function startAuthSync(handlers: Partial<Record<AuthSyncEvent, () => void | Promise<void>>>): () => void {
	if (typeof window === 'undefined') return () => {};

	const handle = (type: unknown) => {
		if (typeof type !== 'string') return;
		void handlers[type as AuthSyncEvent]?.();
	};
	const broadcast = getChannel();
	const onMessage = (event: MessageEvent) => handle(event.data?.type);
	const onStorage = (event: StorageEvent) => {
		if (event.key !== CHANNEL_NAME || !event.newValue) return;
		try {
			handle(JSON.parse(event.newValue).type);
		} catch {
			// ignore malformed events
		}
	};

	broadcast?.addEventListener('message', onMessage);
	window.addEventListener('storage', onStorage);

	return () => {
		broadcast?.removeEventListener('message', onMessage);
		window.removeEventListener('storage', onStorage);
	};
}
