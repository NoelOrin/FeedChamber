import { type NextRequest, NextResponse } from 'next/server';

/*
 * 登录墙的分工（openspec change 2026-10-28-seo-public-indexing, design D4）：
 *
 * proxy 跑在边缘，只能读 cookie，**读不到后端设置**。所以它只负责「结构性私有」
 * ——那些无论内容设置怎么配都必须登录的路径。内容页要不要登录由服务端设置决定，
 * 判定发生在 RSC 层（(app) layout / 各页面），proxy 不参与。
 *
 * 此前这个正则把 /timeline|spaces|t 等公开内容面也锁进来，与后端 AuthGate 的默认
 * 放行语义相反，是「爬虫一条都抓不到」的直接原因。
 */

// 永远私有：私信、发帖入口、聚合首页、账号中心自身主页。
// /user/{id}（他人主页）**不在**此列——它是公开内容，是否进索引由 user_profile_indexable 控制。
const PROTECTED = /^\/(messages|home|timeline\/create)(\/|$)|^\/user\/?$/;

/** Replaces TanStack Router's beforeLoad requireAuth / requireGuest guards. */
export default function proxy(request: NextRequest) {
	const { pathname, search } = request.nextUrl;
	const authed = /(?:^|;\s*)master_auth=1/.test(request.headers.get('cookie') ?? '');

	if (PROTECTED.test(pathname) && !authed) {
		const target = new URL('/login', request.url);
		target.searchParams.set('redirect', pathname + search);
		return NextResponse.redirect(target);
	}

	// 刻意不在 proxy 里把「带标记 cookie」的用户从 /login 弹走：proxy 只能看到
	// `master_auth` 标记，真正的会话要由 `(auth)` layout 的 requireGuest 向服务端确认。
	// 两边都判定会把「后端不可用」放大成 /timeline ↔ /login 的无限跳转。

	// 根布局要按路径决定要不要做外壳取数（/admin 不需要），把路径透传下去。
	const headers = new Headers(request.headers);
	headers.set('x-pathname', pathname);
	return NextResponse.next({ request: { headers } });
}

export const config = {
	// 覆盖所有页面路由：除了挡未登录，还要把 x-pathname 透传给根布局。
	// 登录/注册页的访客判定交给 (auth) layout 的 requireGuest。
	matcher: [
		'/timeline/:path*',
		'/spaces/:path*',
		'/user/:path*',
		'/messages/:path*',
		'/home',
		'/leaderboard',
		'/search',
		'/t/:path*',
		'/admin/:path*',
		'/login',
		'/register',
		'/oauth',
		'/',
	],
};
