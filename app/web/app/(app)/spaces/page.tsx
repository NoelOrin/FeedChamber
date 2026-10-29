import type { Metadata } from 'next';

import { flattenSpaceTree, SpaceRegion } from '~/components/space/space-card';
import { EmptyState } from '~/components/ui/empty-state';
import { PageHeader } from '~/components/ui/page-header';
import { requireAuthBySetting } from '~/lib/auth/guards';
import { loadPublicSettings, loadSpaceTree } from '~/lib/queries/server';
import { listPageMetadataFromSettings } from '~/lib/seo';

export async function generateMetadata(): Promise<Metadata> {
	const settings = await loadPublicSettings();
	return listPageMetadataFromSettings(settings, { title: '版面', path: '/spaces' });
}

/*
 * /spaces —— 版面总览。纯 RSC：卡片本身只有链接，服务端渲染完成即出内容。
 */

export default async function SpacesPage() {
	// 公开内容面的动态登录门：space_view_auth_required=true 时匿名访客跳登录。
	await requireAuthBySetting('space_view_auth_required', '/spaces');

	// 暗区（system / recommendable=false）不进版面大全主推位置：它只在被 exile
	// 的帖子/用户身上通过链接进入，避免软隔离变成可炫耀的猎场（design D3）。
	const all = await loadSpaceTree();
	const spaces = all.filter((node) => node.recommendable !== false);
	const flat = flattenSpaceTree(spaces);

	return (
		<div className="flex w-full flex-col gap-3 pb-10">
			<PageHeader title="版面" meta={`${flat.length} 个`} />

			{spaces.length > 0 ? (
				spaces.map((region) => <SpaceRegion key={region.id} node={region} />)
			) : (
				<EmptyState title="暂无版面" description="管理员可在后台创建新版面及子版面。" />
			)}
		</div>
	);
}
