'use client';

import { usePathname, useRouter } from 'next/navigation';

import { Button } from '~/components/ui/button';
import { EmptyState } from '~/components/ui/empty-state';

/**
 * `/admin/**` 下的未知地址兜底。
 *
 * 为什么不只靠 `not-found.tsx`：主站根级的 404 兜底会落进 `(app)` 布局（带前台壳层且要求登录），
 * `/admin/xxx` 这类「后台里的错地址」会因此跑到前台去。这里用 catch-all 把它留在后台壳层里。
 */
export default function AdminCatchAll() {
	const router = useRouter();
	const pathname = usePathname();

	return (
		<div className="flex min-h-[60vh] flex-col items-center justify-center gap-4">
			<EmptyState title="后台没有这个页面" description={`地址 ${pathname} 不存在，可能已经改名或被移除。`} />
			<Button onClick={() => router.push('/admin')}>返回运营概览</Button>
		</div>
	);
}
