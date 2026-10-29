'use client';

import { Badge } from '~/components/ui/badge';

/**
 * 「已生效 / 草稿」状态徽标。
 *
 * 这是自定义层最重要的一处 UI 语义：写操作返回的 `applied` 必须直白地
 * 呈现出来，否则站长（或 AI）会以为改了就已经上线。`reason` 来自服务端，
 * 通常是「机器凭证未持 customize:publish 权限，已保存为草稿」。
 */
export function AppliedBadge({ applied, reason }: { applied: boolean; reason?: string }) {
	if (applied) {
		return <Badge variant="outline">已生效</Badge>;
	}
	return (
		<span className="inline-flex items-center gap-2">
			<Badge variant="outline">草稿</Badge>
			{reason ? <span className="text-xs text-muted-foreground">{reason}</span> : null}
		</span>
	);
}
