'use client';

import { useRouter } from 'next/navigation';

import ErrorFallback from '~/components/admin/error/ErrorFallback';

/** 路由级错误边界：等价于旧 TanStack Router 的 defaultErrorComponent。 */
export default function AppError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
	const router = useRouter();

	return (
		<ErrorFallback
			error={error}
			reset={reset}
			variant="global"
			onRetry={() => {
				reset();
				router.refresh();
			}}
			onNavigateHome={() => router.push('/admin')}
			onNavigateLogin={() => router.push('/login')}
		/>
	);
}
