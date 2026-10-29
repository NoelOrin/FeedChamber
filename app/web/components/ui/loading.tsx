import { LoaderCircle } from '~/lib/icons';
import { cn } from '~/lib/utils';

export function Loading({ text = '加载中', className }: { text?: string; className?: string }) {
	return (
		<div role="status" className={cn('flex w-full items-center justify-center gap-2 py-16', className)}>
			<LoaderCircle className="size-4 animate-spin text-muted-foreground" aria-hidden="true" />
			<span className="text-xs text-muted-foreground">{text}</span>
		</div>
	);
}
