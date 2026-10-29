'use client';

import { oauthService } from '@orinforum/api-client';
import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';

import { Button } from '~/components/ui/button';
import { Loading } from '~/components/ui/loading';
import { authClient } from '~/lib/auth';

/**
 * OAuth 回调落地：后端在 /oauth/{provider}/callback 的响应里用 Set-Cookie 写 HttpOnly 会话，
 * 前端只负责触发、拉 profile、然后整页跳转让服务端守卫看到新 cookie。
 */
export function OAuthCallback({ provider, code, state }: { provider: string; code: string; state: string }) {
	const router = useRouter();
	const [error, setError] = useState('');

	useEffect(() => {
		let cancelled = false;

		(async () => {
			try {
				const result = await oauthService.callback(provider, code, state);
				if (cancelled) return;
				if (result?.access_token || result?.refresh_token) {
					await authClient.loadProfile().catch(() => undefined);
					window.location.assign('/timeline');
				} else if (result?.needs_register) {
					router.push('/register');
				} else {
					window.location.assign('/timeline');
				}
			} catch (err) {
				if (cancelled) return;
				setError((err as Error)?.message || '登录失败，请重试');
			}
		})();

		return () => {
			cancelled = true;
		};
	}, [provider, code, state, router]);

	if (error) {
		return (
			<div className="flex flex-col items-center gap-4 text-center">
				<p className="text-sm text-destructive">{error}</p>
				<Button variant="primary" size="sm" onClick={() => window.location.assign('/login?redirect=%2Ftimeline')}>
					返回登录
				</Button>
			</div>
		);
	}

	return <Loading text={`正在处理 ${provider} 登录...`} />;
}
