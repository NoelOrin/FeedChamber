import Link from 'next/link';

import { Button } from '~/components/ui/button';

/* 根路由 notFound 兜底：内层路由抛 notFound() 时会落在 (app) 布局里（带顶栏与左栏），
   所以这里只画一块紧凑的居中内容，不设 min-h-screen、不加载远程图。 */
export default function NotFound() {
	return (
		<div className="flex w-full flex-col items-center justify-center px-4 py-16 text-center">
			<p className="text-4xl leading-none font-semibold tracking-tight text-muted-foreground">404</p>
			<h1 className="mt-3 text-lg font-semibold tracking-tight text-foreground">找不到页面</h1>
			<p className="mt-1 text-[13px] text-muted-foreground">链接可能已失效，或页面已被移除。</p>
			<Button asChild size="sm" className="mt-5">
				<Link href="/">返回首页</Link>
			</Button>
		</div>
	);
}
