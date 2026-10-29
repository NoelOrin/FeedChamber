import { clearAuthTokens } from '@orinforum/api-client';

export interface SSEMessage {
	data: Record<string, unknown>;
}

export interface SSEClientOptions {
	url: string;
	method?: 'GET' | 'POST';
	/**
	 * Token provider. Required for Bearer-token mode. Optional (may return null)
	 * when `withCredentials: true` — the browser sends the HttpOnly cookie instead.
	 */
	getToken?: () => string | null;
	refreshToken?: () => Promise<boolean>;
	onData: (data: Record<string, unknown>) => void;
	onEventId?: (id: string) => void;
	onError?: (err: { code: number; msg: string }) => void;
	onStatusChange?: (status: 'connecting' | 'connected' | 'disconnected') => void;
	maxRetries?: number;
	/**
	 * When true, the client uses `credentials: 'include'` and does NOT inject
	 * an `Authorization` header — suited for HttpOnly cookie auth where JS
	 * cannot read the token. Default false (Bearer token mode).
	 */
	withCredentials?: boolean;
	resumeOnVisible?: boolean;
}

export class SSEClient {
	private static readonly INITIAL_RETRY_DELAY = 3000;
	private static readonly MAX_RETRY_DELAY = 30000;

	private opts: Required<Pick<SSEClientOptions, 'method' | 'withCredentials'>> & SSEClientOptions;
	private abortController: AbortController | null = null;
	private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
	private retryDelay = SSEClient.INITIAL_RETRY_DELAY;
	private retryCount = 0;
	private running = false;
	private connected = false;
	private manualStop = false;
	private visibilityHandler: (() => void) | null = null;

	constructor(options: SSEClientOptions) {
		this.opts = {
			method: options.method ?? 'POST',
			withCredentials: options.withCredentials ?? false,
			...options,
		};
	}

	isConnected(): boolean {
		return this.connected;
	}

	start(): void {
		if (this.running) return;
		this.manualStop = false;
		this.running = true;
		this.retryDelay = SSEClient.INITIAL_RETRY_DELAY;
		this.retryCount = 0;
		this.connected = false;
		this.opts.onStatusChange?.('connecting');
		this.connect();
		this.installVisibilityResume();
	}

	stop(): void {
		this.manualStop = true;
		this.running = false;
		this.connected = false;
		this.opts.onStatusChange?.('disconnected');
		if (this.reconnectTimer) {
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = null;
		}
		this.abortController?.abort();
		this.abortController = null;
	}

	private installVisibilityResume(): void {
		if (!this.opts.resumeOnVisible || typeof window === 'undefined' || this.visibilityHandler) return;
		this.visibilityHandler = () => {
			if (document.visibilityState === 'visible' && !this.running && !this.manualStop) {
				this.start();
			}
		};
		document.addEventListener('visibilitychange', this.visibilityHandler);
	}

	private async connect(): Promise<void> {
		if (!this.running) return;

		const token = this.opts.getToken?.() ?? null;
		const cookieMode = this.opts.withCredentials;

		if (!cookieMode && !token) {
			// Bearer mode requires a token; skip and schedule reconnect.
			this.scheduleReconnect();
			return;
		}

		this.abortController = new AbortController();

		try {
			const headers: Record<string, string> = {
				Accept: 'text/event-stream',
			};
			if (!cookieMode && token) {
				headers['Authorization'] = `Bearer ${token}`;
			}

			const response = await fetch(this.opts.url, {
				method: this.opts.method,
				headers,
				credentials: cookieMode ? 'include' : 'same-origin',
				signal: this.abortController.signal,
			});

			if (response.status === 401) {
				const refreshed = this.opts.refreshToken ? await this.opts.refreshToken() : false;
				if (refreshed) {
					this.scheduleReconnect();
				} else {
					clearAuthTokens();
					this.stop();
				}
				return;
			}

			if (!response.ok) {
				const body = await response.json().catch(() => null);
				const code = body?.code ?? 5001;
				if (code > 0 && code < 5000) {
					this.opts.onError?.({ code, msg: body?.msg || '未知错误' });
				}
				this.scheduleReconnect();
				return;
			}

			// Connection successful — reset retry delay and count
			this.retryDelay = SSEClient.INITIAL_RETRY_DELAY;
			this.retryCount = 0;
			this.connected = true;
			this.opts.onStatusChange?.('connected');

			const reader = response.body!.getReader();
			const decoder = new TextDecoder();
			let buffer = '';

			while (this.running) {
				const { done, value } = await reader.read();
				if (done) break;

				buffer += decoder.decode(value, { stream: true });
				const lines = buffer.split('\n');
				buffer = lines.pop() || '';

				for (const line of lines) {
					if (line.startsWith('id: ')) {
						this.opts.onEventId?.(line.slice(4).trim());
					}
					if (line.startsWith('data: ')) {
						try {
							const data = JSON.parse(line.slice(6)) as Record<string, unknown>;
							this.opts.onData(data);
						} catch {
							// JSON parse failure — silent
						}
					}
				}
			}
		} catch {
			// Network disconnect or abort — silent
		}

		this.connected = false;
		this.opts.onStatusChange?.('disconnected');
		if (this.running) {
			this.scheduleReconnect();
		}
	}

	private scheduleReconnect(): void {
		if (!this.running) return;
		if (this.reconnectTimer) return;
		this.retryCount++;
		const maxRetries = this.opts.maxRetries ?? 5;
		if (this.retryCount >= maxRetries) {
			this.stop();
			return;
		}
		this.opts.onStatusChange?.('connecting');
		this.reconnectTimer = setTimeout(() => {
			this.reconnectTimer = null;
			this.retryDelay = Math.min(this.retryDelay * 1.5, SSEClient.MAX_RETRY_DELAY);
			this.connect();
		}, this.retryDelay);
	}
}
