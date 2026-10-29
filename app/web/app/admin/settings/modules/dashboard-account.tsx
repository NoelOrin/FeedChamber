'use client';

import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { useAuthStore } from '~/lib/auth';

/* 只读邮箱展示：原来是 antd Form 的 disabled Input，实际没有任何表单行为，
   换成一个 Label + 只读 Input 更贴合语义。 */
export function DashboardAccount() {
	const user = useAuthStore((s) => s.user);

	return (
		<div className="flex max-w-[400px] flex-col gap-1.5">
			<Label htmlFor="admin-account-email">邮箱</Label>
			<Input id="admin-account-email" value={user?.email || '-'} disabled />
		</div>
	);
}
