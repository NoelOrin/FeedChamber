'use client';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { trustLevelName, useTrustLevels } from '~/lib/hooks/use-trust-levels';
import { PenLine } from '~/lib/icons';
import { cn } from '~/lib/utils';

interface ProfileHeaderProps {
	displayName: string;
	role?: string;
	trustLevel?: number | string;
	avatar: string;
	isOwnProfile: boolean;
	onEdit?: () => void;
	followCounts: {
		followers: number;
		following: number;
	};
	className?: string;
}

const ROLE_LABELS: Record<string, string> = {
	admin: '管理员',
	moderator: '版主',
	user: '成员',
	ban: '已封禁',
};

export function ProfileHeader({
	displayName,
	role,
	trustLevel,
	avatar,
	isOwnProfile,
	onEdit,
	followCounts,
	className,
}: ProfileHeaderProps) {
	const roleLabel = role ? (ROLE_LABELS[role] ?? role) : '';
	const levels = useTrustLevels();
	const trustLabelText = trustLevelName(levels, trustLevel);

	return (
		<div className={cn('flex flex-col gap-4 sm:flex-row sm:items-center', className)}>
			<div className="grid size-16 shrink-0 place-items-center overflow-hidden rounded-full border border-border bg-muted text-lg font-medium text-muted-foreground">
				{avatar ? (
					<img src={avatar} alt="" loading="lazy" className="size-full object-cover" />
				) : (
					(displayName || '?').slice(0, 1).toUpperCase()
				)}
			</div>

			<div className="min-w-0 flex-1">
				<div className="flex flex-wrap items-center gap-2">
					<h1 className="truncate text-xl font-semibold tracking-tight text-foreground">{displayName}</h1>
					{roleLabel ? <Badge variant="state">{roleLabel}</Badge> : null}
					{trustLabelText ? <Badge variant="secondary">{trustLabelText}</Badge> : null}
				</div>

				<div className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
					<span>
						<span className="font-medium text-foreground tabular-nums">{followCounts.following}</span> 关注
					</span>
					<span>
						<span className="font-medium text-foreground tabular-nums">{followCounts.followers}</span> 粉丝
					</span>
				</div>
			</div>

			{isOwnProfile && onEdit ? (
				<Button size="sm" variant="outline" className="shrink-0 gap-1.5 self-start sm:self-center" onClick={onEdit}>
					<PenLine className="size-3.5" aria-hidden="true" />
					编辑资料
				</Button>
			) : null}
		</div>
	);
}
