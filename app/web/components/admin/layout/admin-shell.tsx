'use client';

import { useQueryClient } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { useAccessPermissions } from '~/components/admin/access/access-context';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipTrigger } from '~/components/ui/tooltip';
import { canAccess } from '~/lib/admin/auth/access';
import { requiredPermissionsFor } from '~/lib/admin/auth/route-access';
import { useLocation, useNavigate } from '~/lib/admin/navigation';
import { authClient } from '~/lib/auth';
import { useLocale } from '~/lib/i18n';
import {
	Api,
	Appstore,
	ChevronDown,
	Database,
	FileSearch,
	FileText,
	Flag,
	Globe,
	Key,
	Languages,
	LayoutDashboard,
	LogOut,
	MessageSquare,
	Palette,
	PanelLeftClose,
	PanelLeftOpen,
	Safety,
	Settings,
	Shield,
	Sparkles,
	Tags,
	Trash2,
	Trophy,
	User,
	Users,
} from '~/lib/icons';
import { useSiteName } from '~/lib/settings';
import { useTheme } from '~/lib/theme';
import { cn } from '~/lib/utils';

/*
 * 后台壳层：侧栏菜单 + 顶栏（主题/语言/用户/登出）+ 内容区。
 *
 * 结构对齐主站 AppShell（描边侧栏、56px 顶栏、无投影），而不是 ProLayout
 * 的品牌色头部 + 卡片投影。菜单树与权限门控逻辑从旧 ProShell 平移，只把
 * antd 的渲染层（ProLayout / Menu / Dropdown / Avatar / Popover）换成 shadcn 原语。
 *
 * 为什么不用主站 AppShell 组件本身：它的侧栏是社区导航（时间线/版面/标签），
 * 与后台菜单是两套数据源；共用只会引入一个「传哪套菜单」的开关。这里复用它的
 * 布局几何与视觉语言，菜单数据源独立。
 */

interface MenuItem {
	path: string;
	name: string;
	icon?: ReactNode;
	hideInMenu?: boolean;
	children?: MenuItem[];
}

type AdminUser = { username?: string; email?: string; avatar?: string };

const API_BASE = '/api/v1';

function resolveAvatarUrl(url: string): string {
	if (url.startsWith('http://') || url.startsWith('https://')) return url;
	if (url.startsWith('/')) {
		if (url.startsWith(`${API_BASE}/`)) return url;
		return API_BASE + url;
	}
	return url;
}

/** 叶子菜单是否可见：按该路径的权限要求判断（未登记路径不设门控）。 */
function canAccessMenu(path: string, permissions: readonly string[], permissionsLoaded: boolean): boolean {
	return canAccess(permissions, permissionsLoaded, requiredPermissionsFor(path));
}

export function AdminShell({ children, user }: { children?: ReactNode; user?: AdminUser | null }) {
	const navigate = useNavigate();
	const location = useLocation();
	const queryClient = useQueryClient();
	const { locale: language, setLocale: changeLanguage } = useLocale();
	const siteName = useSiteName();
	const { mode, cycle: cycleTheme, title: themeTitle } = useTheme();
	const { permissions, permissionsLoaded } = useAccessPermissions();

	const [collapsed, setCollapsed] = useState(false);
	const [openGroups, setOpenGroups] = useState<string[]>(() => {
		try {
			const stored = localStorage.getItem('menu_open_keys');
			return stored ? JSON.parse(stored) : [];
		} catch {
			return [];
		}
	});

	useEffect(() => {
		localStorage.setItem('menu_open_keys', JSON.stringify(openGroups));
	}, [openGroups]);

	useEffect(() => {
		document.title = `${siteName} 后台管理`;
	}, [siteName]);

	const pathname = location.pathname;

	const menuItems = useMemo<MenuItem[]>(
		() => [
			{ path: '/admin', name: '运营概览', icon: <LayoutDashboard className="size-4" /> },
			{
				path: '/admin/group/forum',
				name: '论坛管理',
				icon: <Appstore className="size-4" />,
				children: [
					{ path: '/admin/banned', name: '禁言管理', icon: <Safety className="size-4" /> },
					{ path: '/admin/posts', name: '帖子管理', icon: <FileText className="size-4" /> },
					{ path: '/admin/posts/detail', name: '帖子详情', icon: <FileText className="size-4" />, hideInMenu: true },
					{ path: '/admin/comments', name: '评论管理', icon: <MessageSquare className="size-4" /> },
					{
						path: '/admin/comments/detail',
						name: '评论详情',
						icon: <MessageSquare className="size-4" />,
						hideInMenu: true,
					},
					{ path: '/admin/trash', name: '回收站', icon: <Trash2 className="size-4" /> },
					{ path: '/admin/tags', name: '标签管理', icon: <Tags className="size-4" /> },
				],
			},
			{
				path: '/admin/group/operation',
				name: '运营管理',
				icon: <Flag className="size-4" />,
				children: [
					{ path: '/admin/announcements', name: '公告管理', icon: <Flag className="size-4" /> },
					{ path: '/admin/reports', name: '举报管理', icon: <Flag className="size-4" /> },
					{ path: '/admin/moderation', name: '内容审核', icon: <Safety className="size-4" /> },
					{
						path: '/admin/reports/detail',
						name: '举报详情',
						icon: <Flag className="size-4" />,
						hideInMenu: true,
					},
					{ path: '/admin/spaces', name: '版面管理', icon: <Appstore className="size-4" /> },
				],
			},
			{
				path: '/admin/group/customize',
				name: '站点自定义',
				icon: <Palette className="size-4" />,
				children: [
					{ path: '/admin/customize/snippets', name: '自定义代码', icon: <FileText className="size-4" /> },
					{ path: '/admin/customize/theme', name: '主题覆盖', icon: <Palette className="size-4" /> },
					{ path: '/admin/customize/nav', name: '导航与入口', icon: <Appstore className="size-4" /> },
					{ path: '/admin/customize/home', name: '首页模块', icon: <LayoutDashboard className="size-4" /> },
					{ path: '/admin/customize/pages', name: '自定义页面', icon: <FileText className="size-4" /> },
				],
			},
			{
				path: '/admin/group/user-perm',
				name: '用户与权限',
				icon: <Users className="size-4" />,
				children: [
					{ path: '/admin/users', name: '用户列表', icon: <User className="size-4" /> },
					{ path: '/admin/invites', name: '邀请码', icon: <Key className="size-4" /> },
					{ path: '/admin/users/detail', name: '用户详情', icon: <User className="size-4" />, hideInMenu: true },
					{ path: '/admin/roles', name: '角色权限', icon: <Users className="size-4" /> },
					{ path: '/admin/roles/detail', name: '角色详情', icon: <Users className="size-4" />, hideInMenu: true },
					{ path: '/admin/groups', name: '用户组', icon: <Users className="size-4" /> },
					{ path: '/admin/levels', name: '等级定义', icon: <Trophy className="size-4" /> },
					{ path: '/admin/xp-rules', name: 'XP 规则', icon: <Trophy className="size-4" /> },
					{ path: '/admin/gates', name: '门控规则', icon: <Shield className="size-4" /> },
				],
			},
			{
				path: '/admin/group/settings',
				name: '系统设置',
				icon: <Settings className="size-4" />,
				children: [
					{ path: '/admin/api-keys', name: 'API Key', icon: <Key className="size-4" /> },
					{ path: '/admin/webhooks', name: 'Webhook', icon: <Api className="size-4" /> },
					{ path: '/admin/backups', name: '备份', icon: <Database className="size-4" /> },
					{ path: '/admin/oauth-clients', name: 'OAuth 客户端', icon: <Safety className="size-4" /> },
					{ path: '/admin/plugins', name: '插件', icon: <Appstore className="size-4" /> },
					{ path: '/admin/mcp', name: 'MCP 工具', icon: <Sparkles className="size-4" /> },
					{ path: '/admin/mcp-servers', name: '外部 MCP', icon: <Sparkles className="size-4" /> },
					{ path: '/admin/security', name: '安全事件', icon: <Safety className="size-4" /> },
					{ path: '/admin/sessions', name: '会话管理', icon: <Key className="size-4" /> },
					{ path: '/admin/settings', name: '服务设置', icon: <Settings className="size-4" /> },
					{ path: '/admin/audit-logs', name: '审计日志', icon: <FileSearch className="size-4" /> },
				],
			},
		],
		[],
	);

	// 菜单只展示当前角色有权限打开的模块；整组都无权时连组一起收起。
	const visibleMenuItems = useMemo(() => {
		const keep = (item: MenuItem): boolean => {
			if (item.children) return item.children.some(keep);
			return canAccessMenu(item.path, permissions, permissionsLoaded);
		};
		return menuItems
			.filter(keep)
			.map((item) => (item.children ? { ...item, children: item.children.filter(keep) } : item));
	}, [menuItems, permissions, permissionsLoaded]);

	// 当前路径所属的组在**导航时**自动展开，避免刷新后菜单收起找不到自己。
	//
	// 这里刻意只认 pathname 变化：早前依赖数组里带了 openGroups，于是用户刚把
	// 当前所在的组手动收起，effect 立刻把它又展开——「父级收不起来」。
	// 用 ref 记住上一次已经自动展开过的路径，同一路径下的开合完全交给用户；
	// 权限码尚未到货（visibleMenuItems 还空着）时先不记账，等菜单成形再展开一次。
	const lastAutoOpenPath = useRef<string | null>(null);
	useEffect(() => {
		if (lastAutoOpenPath.current === pathname) return;
		const owner = visibleMenuItems.find((item) => item.children?.some((child) => pathname.startsWith(child.path)));
		if (!owner) return;
		lastAutoOpenPath.current = pathname;
		setOpenGroups((prev) => (prev.includes(owner.path) ? prev : [...prev, owner.path]));
	}, [pathname, visibleMenuItems]);

	// 命中判定用「最长前缀胜出」：`/admin` 是前缀一切的根，裸 startsWith 会让
	// 「运营概览」在每个后台页都亮着，也会让 `/admin/posts` 与 `/admin/posts/detail`
	// 同时高亮。这里只让最具体的那个**在菜单里真的渲染出来**的项亮——详情页是
	// `hideInMenu`，把它算进「更具体」会让列表项和详情项都不亮。
	const isActive = useCallback(
		(path: string) => {
			const matches = (p: string) => pathname === p || pathname.startsWith(`${p}/`);
			if (!matches(path)) return false;
			const deeper = visibleMenuItems.some((item) =>
				(item.children ?? [item]).some(
					(child) =>
						!child.hideInMenu && child.path !== path && matches(child.path) && child.path.startsWith(`${path}/`),
				),
			);
			return !deeper;
		},
		[pathname, visibleMenuItems],
	);

	async function handleLogout() {
		await authClient.logout();
		queryClient.clear();
		window.location.assign('/login');
	}

	const iconButton =
		'inline-flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/80 focus-visible:outline-none';

	// 单页应用壳层：整页高度锁死，只有内容区（main）滚动。
	// 早前是 `min-h-screen` 文档流布局，页面一长就把侧栏和顶栏一起滚走。
	return (
		<div className="admin-surface bg-background text-foreground flex h-screen overflow-hidden">
			{/* ── 侧栏 ───────────────────────────────────────────── */}
			<aside
				className={cn(
					'border-border bg-card flex h-full shrink-0 flex-col border-r transition-[width] duration-200',
					collapsed ? 'w-[60px]' : 'w-[228px]',
				)}
			>
				<div className={cn('flex h-14 items-center border-b border-border', collapsed ? 'justify-center' : 'px-4')}>
					<span className={cn('truncate font-semibold text-foreground', collapsed ? 'text-sm' : 'text-[15px]')}>
						{collapsed ? siteName.slice(0, 1) : siteName}
					</span>
					{!collapsed ? (
						<Badge variant="id" className="ml-2 shrink-0">
							后台
						</Badge>
					) : null}
				</div>

				<nav className="flex-1 overflow-y-auto px-2 py-3">
					{visibleMenuItems.map((item) => {
						if (!item.children) {
							return (
								<Tooltip key={item.path} delayDuration={collapsed ? 100 : 99999}>
									<TooltipTrigger asChild>
										<button
											type="button"
											onClick={() => navigate({ to: item.path })}
											className={cn(
												'flex w-full cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-[13px] transition-colors',
												collapsed && 'justify-center',
												isActive(item.path)
													? 'bg-accent font-medium text-brand-text'
													: 'text-muted-foreground hover:bg-muted hover:text-foreground',
											)}
										>
											{item.icon}
											{!collapsed ? <span className="truncate">{item.name}</span> : null}
										</button>
									</TooltipTrigger>
									{collapsed ? <TooltipContent side="right">{item.name}</TooltipContent> : null}
								</Tooltip>
							);
						}

						const open = openGroups.includes(item.path);
						const groupActive = item.children.some((child) => isActive(child.path));
						return (
							<div key={item.path} className="mt-1">
								<button
									type="button"
									onClick={() =>
										setOpenGroups((prev) =>
											prev.includes(item.path) ? prev.filter((p) => p !== item.path) : [...prev, item.path],
										)
									}
									className={cn(
										'flex w-full cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-[13px] transition-colors',
										collapsed && 'justify-center',
										groupActive ? 'text-foreground' : 'text-muted-foreground hover:bg-muted hover:text-foreground',
									)}
								>
									{item.icon}
									{!collapsed ? (
										<>
											<span className="flex-1 truncate text-left">{item.name}</span>
											<ChevronDown className={cn('size-3.5 shrink-0 transition-transform', open && 'rotate-180')} />
										</>
									) : null}
								</button>
								{open && !collapsed ? (
									<div className="mt-0.5 ml-3 flex flex-col gap-0.5 border-l border-border pl-2">
										{item.children
											.filter((child) => !child.hideInMenu)
											.map((child) => (
												<button
													key={child.path}
													type="button"
													onClick={() => navigate({ to: child.path })}
													className={cn(
														'flex w-full cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-[13px] transition-colors',
														isActive(child.path)
															? 'bg-accent font-medium text-brand-text'
															: 'text-muted-foreground hover:bg-muted hover:text-foreground',
													)}
												>
													{child.icon}
													<span className="truncate">{child.name}</span>
												</button>
											))}
									</div>
								) : null}
							</div>
						);
					})}
				</nav>

				{/* 侧栏底部：语言 / 主题 / 折叠 */}
				<div className={cn('border-t border-border p-2', collapsed ? 'flex flex-col gap-1' : 'space-y-1')}>
					{collapsed ? (
						<>
							<Tooltip>
								<TooltipTrigger asChild>
									<button
										type="button"
										className={iconButton}
										onClick={() => changeLanguage(language === 'zh-CN' ? 'en-US' : 'zh-CN')}
									>
										<Languages className="size-4" />
									</button>
								</TooltipTrigger>
								<TooltipContent side="right">{language === 'zh-CN' ? '中文' : 'English'}</TooltipContent>
							</Tooltip>
							<Tooltip>
								<TooltipTrigger asChild>
									<button type="button" className={iconButton} onClick={() => cycleTheme()} title={themeTitle}>
										<Globe className="size-4" />
									</button>
								</TooltipTrigger>
								<TooltipContent side="right">{themeTitle}</TooltipContent>
							</Tooltip>
						</>
					) : (
						<>
							<button
								type="button"
								onClick={() => changeLanguage(language === 'zh-CN' ? 'en-US' : 'zh-CN')}
								className="flex w-full cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
							>
								<Languages className="size-3.5" />
								<span>{language === 'zh-CN' ? '中文' : 'English'}</span>
							</button>
							<button
								type="button"
								onClick={() => cycleTheme()}
								title={themeTitle}
								className="flex w-full cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
							>
								{/* 三个图标都由 CSS 按 html[data-theme-mode] 显隐，见 admin.css */}
								<svg data-theme-icon="light" className="size-3.5" fill="none" viewBox="0 0 48 48" aria-hidden="true">
									<circle cx="24" cy="24" r="11" stroke="currentColor" strokeWidth="3" />
									<path
										d="M24 4v4M24 40v4M4 24h4M40 24h4M11.4 11.4l2.8 2.8M33.8 33.8l2.8 2.8M11.4 36.6l2.8-2.8M33.8 14.2l2.8-2.8"
										stroke="currentColor"
										strokeWidth="3"
										strokeLinecap="round"
									/>
								</svg>
								<svg data-theme-icon="dark" className="size-3.5" fill="none" viewBox="0 0 48 48" aria-hidden="true">
									<path
										d="M24 4C12.96 4 4 12.96 4 24s8.96 20 20 20c.89 0 1.76-.06 2.62-.18C19.7 41.09 17 34.76 17 28c0-7.73 6.27-14 14-14 .58 0 1.15.04 1.72.1C35.2 8.76 30.36 4 24 4z"
										stroke="currentColor"
										strokeWidth="3"
										strokeLinejoin="round"
									/>
								</svg>
								<svg data-theme-icon="system" className="size-3.5" fill="none" viewBox="0 0 24 24" aria-hidden="true">
									<rect x="2" y="3" width="20" height="14" rx="2" stroke="currentColor" strokeWidth="2" />
									<path d="M8 21h8M12 17v4" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
								</svg>
								<span>{themeTitle}</span>
							</button>
						</>
					)}
					<button
						type="button"
						onClick={() => setCollapsed((v) => !v)}
						className={cn(iconButton, collapsed ? 'self-center' : 'w-full justify-start gap-2 px-2 text-xs')}
						aria-label={collapsed ? '展开侧栏' : '收起侧栏'}
					>
						{collapsed ? <PanelLeftOpen className="size-4" /> : <PanelLeftClose className="size-4" />}
						{!collapsed ? <span>收起</span> : null}
					</button>
				</div>
			</aside>

			{/* ── 主区 ───────────────────────────────────────────── */}
			<div className="flex h-full min-w-0 flex-1 flex-col overflow-hidden">
				<header className="border-border bg-card flex h-14 shrink-0 items-center justify-between gap-3 border-b px-6">
					<div className="flex min-w-0 items-center gap-2">
						<Button variant="ghost" size="sm" onClick={() => navigate({ to: '/admin' })}>
							后台管理
						</Button>
					</div>

					<div className="flex items-center gap-2">
						<Tooltip>
							<TooltipTrigger asChild>
								<button type="button" className={iconButton} onClick={() => cycleTheme()} title={themeTitle}>
									<span className="text-xs">{mode === 'light' ? '亮' : mode === 'dark' ? '暗' : '随'}</span>
								</button>
							</TooltipTrigger>
							<TooltipContent>{themeTitle}</TooltipContent>
						</Tooltip>

						<DropdownMenu>
							<DropdownMenuTrigger asChild>
								<button
									type="button"
									className="flex cursor-pointer items-center gap-1.5 rounded-md px-1.5 py-1 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
								>
									<Languages className="size-3.5" />
									{language === 'zh-CN' ? '中文' : 'English'}
									<ChevronDown className="size-3" />
								</button>
							</DropdownMenuTrigger>
							<DropdownMenuContent align="end">
								<DropdownMenuItem onClick={() => changeLanguage('zh-CN')}>中文</DropdownMenuItem>
								<DropdownMenuItem onClick={() => changeLanguage('en-US')}>English</DropdownMenuItem>
							</DropdownMenuContent>
						</DropdownMenu>

						<DropdownMenu>
							<DropdownMenuTrigger asChild>
								<button
									type="button"
									className="flex cursor-pointer items-center gap-1.5 rounded-md px-1.5 py-1 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
								>
									<span className="bg-muted flex size-6 shrink-0 items-center justify-center overflow-hidden rounded-full">
										{user?.avatar ? (
											<img src={resolveAvatarUrl(user.avatar)} alt="" className="size-full object-cover" />
										) : (
											<User className="size-3.5" />
										)}
									</span>
									<span className="max-w-28 truncate">{user?.username || user?.email || '用户'}</span>
									<ChevronDown className="size-3" />
								</button>
							</DropdownMenuTrigger>
							<DropdownMenuContent align="end">
								<DropdownMenuItem onClick={handleLogout}>
									<LogOut className="size-3.5" />
									退出登录
								</DropdownMenuItem>
							</DropdownMenuContent>
						</DropdownMenu>
					</div>
				</header>

				{/* 内容区是唯一的滚动容器（侧栏与顶栏固定）。 */}
				<main id="main-content" tabIndex={-1} className="min-w-0 flex-1 overflow-y-auto px-6 py-6 outline-none">
					<div className="mx-auto w-full max-w-[1280px]">{children}</div>
				</main>
			</div>
		</div>
	);
}
