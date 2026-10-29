'use client';

import { useState } from 'react';

import { Button } from '~/components/ui/button';
import { authClient } from '~/lib/auth';
import { Lock, LogOut, RefreshCw } from '~/lib/icons';

interface Props {
	/** 已登录用户的展示名，未登录时不传。 */
	username?: string;
	role?: string;
	detail?: string;
	onRetry?: () => void;
}

/**
 * 403 无权限：已登录但不够格进后台时，整页替换壳层——不渲染菜单、不加载任何后台数据。
 */
export function AccessDenied({ username, role, detail, onRetry }: Props) {
	const [loggingOut, setLoggingOut] = useState(false);

	async function handleLogout() {
		setLoggingOut(true);
		try {
			await authClient.logout();
		} finally {
			// 整页跳转：让服务端守卫看到已清掉的 cookie。
			window.location.assign('/login');
		}
	}

	return (
		<div className="bg-background flex min-h-screen w-full items-center justify-center px-6">
			<div className="border-border bg-card w-full max-w-[560px] rounded-lg border px-8 py-10">
				<div className="flex flex-col items-center gap-3 text-center">
					<span className="bg-destructive/10 text-destructive flex size-12 items-center justify-center rounded-full">
						<Lock className="size-5" />
					</span>
					<h1 className="text-xl font-semibold tracking-tight text-foreground">403 无权限</h1>
					<p className="text-muted-foreground max-w-[420px] text-[13px]">
						当前账号没有管理后台的访问权限。如需进入，请联系管理员为你的账号分配管理员或版主角色。
					</p>
					<div className="mt-1 flex flex-wrap items-center justify-center gap-2">
						<Button size="sm" loading={loggingOut} onClick={handleLogout}>
							<LogOut className="size-3.5" />
							退出登录
						</Button>
						{onRetry ? (
							<Button variant="outline" size="sm" onClick={onRetry}>
								<RefreshCw className="size-3.5" />
								重新检测
							</Button>
						) : null}
					</div>
				</div>

				<div className="border-border mt-6 space-y-1 border-t pt-4 text-center">
					{username ? (
						<p className="text-muted-foreground text-[13px]">
							当前账号：<strong className="font-medium text-foreground">{username}</strong>
							{role ? `（${role}）` : ''}
						</p>
					) : null}
					{detail ? <p className="text-muted-foreground text-xs">{detail}</p> : null}
					<p className="text-muted-foreground text-xs">若认为这是误判，请向站点管理员提供你的用户名。</p>
				</div>
			</div>
		</div>
	);
}
