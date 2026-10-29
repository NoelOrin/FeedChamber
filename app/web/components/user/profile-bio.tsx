import { Textarea } from '~/components/ui/textarea';
import { cn } from '~/lib/utils';

interface ProfileBioProps {
	bio: string;
	editing?: boolean;
	value?: string;
	onChange?: (v: string) => void;
	className?: string;
}

/** 简介栏：纯文本呈现（正文渲染统一走 MarkdownRenderer，这里不新增注入路径）。 */
export function ProfileBio({ bio, editing, value, onChange, className }: ProfileBioProps) {
	return (
		<section className={cn('flex flex-col gap-2', className)}>
			<h2 className="text-xs font-medium text-muted-foreground">个人简介</h2>
			{editing ? (
				<Textarea
					rows={4}
					maxLength={200}
					placeholder="一百字内介绍自己，支持 Markdown"
					value={value ?? ''}
					onChange={(event) => onChange?.(event.currentTarget.value)}
				/>
			) : bio.trim().length > 0 ? (
				<p className="text-sm leading-7 break-words whitespace-pre-wrap text-foreground">{bio}</p>
			) : (
				<p className="text-sm text-muted-foreground">还没有填写简介。</p>
			)}
		</section>
	);
}
