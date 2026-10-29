import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import type { LucideIcon } from '~/lib/icons';
import { AlertCircle } from '~/lib/icons';

/** 认证卡里的主行动按钮：登录/注册同一套尺寸。 */
export const authSubmitClass = 'mt-1 h-9 w-full gap-1.5';

/** 表单级错误条。 */
export function AuthError({ message }: { message: string }) {
	if (!message) return null;
	return (
		<div
			role="alert"
			className="flex items-start gap-2 rounded-md border border-destructive/25 bg-destructive/5 px-3 py-2 text-xs text-destructive"
		>
			<AlertCircle size={14} className="mt-px shrink-0" aria-hidden="true" />
			<span className="leading-5">{message}</span>
		</div>
	);
}

/** 带左侧图标的输入框：标签在框外上方。 */
export function AuthField({
	id,
	label,
	icon: Icon,
	required,
	...props
}: Omit<React.ComponentProps<'input'>, 'id'> & { id: string; label: string; icon: LucideIcon; required?: boolean }) {
	return (
		<div className="flex flex-col gap-1.5">
			<Label htmlFor={id}>{label}</Label>
			<div className="relative">
				<Icon
					size={15}
					aria-hidden="true"
					className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-muted-foreground"
				/>
				<Input id={id} required={required} className="h-9 pl-9" {...props} />
			</div>
		</div>
	);
}
