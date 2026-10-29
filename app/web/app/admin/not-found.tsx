'use client';

import { useRouter } from 'next/navigation';

import { Button } from '~/components/ui/button';
import { EmptyState } from '~/components/ui/empty-state';

/** 404：等价于旧 TanStack Router 的 defaultNotFoundComponent。 */
export default function NotFound() {
	const router = useRouter();

	return (
		<div className="flex min-h-[60vh] flex-col items-center justify-center gap-4">
			<EmptyState title="404 · 页面不存在" description="抱歉，您访问的页面不存在。" />
			<Button onClick={() => router.push('/admin')}>返回首页</Button>
		</div>
	);
}
