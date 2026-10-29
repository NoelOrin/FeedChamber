import type * as React from 'react';

import { requireGuest } from '~/lib/auth/guards';

/** 旧 (auth)/route.tsx 的 requireGuest：已登录访问登录/注册页直接回时间线。 */
export default async function AuthLayout({ children }: { children: React.ReactNode }) {
	await requireGuest();
	return children;
}
