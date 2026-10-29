'use client';

import { useState } from 'react';

import { AuthError, AuthField, authSubmitClass } from '~/components/auth/auth-form-shell';
import { Button } from '~/components/ui/button';
import { authClient } from '~/lib/auth';
import { Lock, User } from '~/lib/icons';
import { toast } from '~/lib/toast';

/** 只允许站内相对路径，挡掉 `//evil.com` 与 `/\evil.com` 这类开放重定向。 */
function safeRedirect(value: string | undefined) {
	if (!value || value === 'undefined') return null;
	if (!value.startsWith('/') || value.startsWith('//') || value.startsWith('/\\')) return null;
	return value;
}

export function LoginForm({ redirect }: { redirect?: string }) {
	const [identity, setIdentity] = useState('');
	const [password, setPassword] = useState('');
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState('');

	async function onSubmit(event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault();
		const login = identity.trim();
		if (!login || !password) {
			setError('请输入账号和密码');
			return;
		}
		setError('');
		setLoading(true);
		try {
			await authClient.login({ login, password });
			toast.success('登录成功');
			// 整页跳转：新的 httpOnly cookie 要让服务端守卫看到
			window.location.assign(safeRedirect(redirect) ?? '/timeline');
		} catch (err) {
			toast.error((err as Error)?.message || '登录失败，请检查用户名或密码');
			setLoading(false);
		}
	}

	return (
		<form onSubmit={onSubmit} className="flex w-full flex-col gap-4">
			<AuthField
				id="login-identity"
				label="账号"
				icon={User}
				type="text"
				value={identity}
				onChange={(event) => setIdentity(event.currentTarget.value)}
				placeholder="用户名 / 邮箱"
				autoComplete="username"
				required
			/>
			<AuthField
				id="login-password"
				label="密码"
				icon={Lock}
				type="password"
				value={password}
				onChange={(event) => setPassword(event.currentTarget.value)}
				placeholder="••••••••"
				autoComplete="current-password"
				required
			/>
			<AuthError message={error} />
			<Button type="submit" variant="primary" loading={loading} className={authSubmitClass}>
				登录
			</Button>
			<div className="text-center text-[11px] text-muted-foreground">加密传输 · 安全登录</div>
		</form>
	);
}
