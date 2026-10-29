'use client';

import { Button } from '~/components/ui/button';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { Textarea } from '~/components/ui/textarea';
import { cn } from '~/lib/utils';

interface ProfileEditFormProps {
	displayName: string;
	username: string;
	bio: string;
	onDisplayNameChange: (v: string) => void;
	onUsernameChange: (v: string) => void;
	onBioChange: (v: string) => void;
	onCancel: () => void;
	onSave: () => void;
	saving: boolean;
	className?: string;
}

export function ProfileEditForm({
	displayName,
	username,
	bio,
	onDisplayNameChange,
	onUsernameChange,
	onBioChange,
	onCancel,
	onSave,
	saving,
	className,
}: ProfileEditFormProps) {
	return (
		<div className={cn('rounded-lg border border-border bg-muted/30 p-4', className)}>
			<div className="grid gap-4 sm:grid-cols-2">
				<div className="flex flex-col gap-1.5">
					<Label htmlFor="profile-display-name">显示名称</Label>
					<Input
						id="profile-display-name"
						value={displayName}
						onChange={(event) => onDisplayNameChange(event.currentTarget.value)}
						maxLength={32}
						placeholder="例：林小满"
					/>
				</div>

				<div className="flex flex-col gap-1.5">
					<Label htmlFor="profile-username">用户名</Label>
					<Input
						id="profile-username"
						value={username}
						onChange={(event) => onUsernameChange(event.currentTarget.value)}
						maxLength={32}
						placeholder="username"
					/>
				</div>

				<div className="flex flex-col gap-1.5 sm:col-span-2">
					<Label htmlFor="profile-bio">个人简介</Label>
					<Textarea
						id="profile-bio"
						rows={4}
						placeholder="一句话介绍自己，支持 Markdown。"
						value={bio}
						onChange={(event) => onBioChange(event.currentTarget.value)}
						maxLength={200}
					/>
					<span className="text-right text-[11px] text-muted-foreground tabular-nums">{bio.length} / 200</span>
				</div>
			</div>

			<div className="mt-4 flex justify-end gap-2">
				<Button size="sm" variant="outline" onClick={onCancel} disabled={saving}>
					取消
				</Button>
				<Button size="sm" loading={saving} onClick={onSave} disabled={saving}>
					保存资料
				</Button>
			</div>
		</div>
	);
}
