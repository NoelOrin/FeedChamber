import type { ReactNode, SuspenseProps } from 'react';
import { Suspense } from 'react';

import { UserProfileSkeleton } from '~/components/user/skeletons';

/** 旧 `/(app)/user/route.tsx`：用户中心整块共用一个 Suspense 边界，fallback 是账号卡骨架。 */
export default function UserRouteLayout({ children }: { children: ReactNode }) {
	const fallback: SuspenseProps['fallback'] = (
		<div className="w-full">
			<UserProfileSkeleton />
		</div>
	);
	return <Suspense fallback={fallback}>{children}</Suspense>;
}
