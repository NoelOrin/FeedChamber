'use client';

import { useRouter } from 'next/navigation';
import { useEffect } from 'react';

/** 旧路由 `/settings/dashboard` 的兼容跳转：统一落到带 tab 参数的设置页。 */
export default function SettingsDashboardRedirect() {
	const router = useRouter();

	useEffect(() => {
		router.replace('/admin/settings?tab=dashboard');
	}, [router]);

	return null;
}
