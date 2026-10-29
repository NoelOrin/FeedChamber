import type { Metadata } from 'next';

import { MotionHall } from '~/components/motion/motion-hall';
import { requireAuth } from '~/lib/auth/guards';
import { loadPublicSettings } from '~/lib/queries/server';
import { listPageMetadataFromSettings } from '~/lib/seo';

export async function generateMetadata(): Promise<Metadata> {
	const settings = await loadPublicSettings();
	return listPageMetadataFromSettings(settings, { title: '自治大厅', path: '/motions' });
}

/*
 * /motions —— 社区自治大厅。动议是灰区的集体裁判通道，与 /reports 的举报分流。
 * 表决面板与发起入口均在客户端组件里（useQuery + useMutation）。
 */
export default async function MotionsPage() {
	const user = await requireAuth('/motions');

	// 动议的 manage 操作（翻案/滥诉/硬封禁升级）跟随既有 moderator/admin 角色。
	const canManage = user.role === 'admin' || user.role === 'moderator';

	return <MotionHall canManage={canManage} />;
}
