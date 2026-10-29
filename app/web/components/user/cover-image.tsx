'use client';

import { Button } from '~/components/ui/button';
import { CoverImageSkeleton } from '~/components/user/skeletons';
import { Image as ImageIcon } from '~/lib/icons';
import { toast } from '~/lib/toast';
import { cn } from '~/lib/utils';

interface CoverImageProps {
	url?: string;
	editing?: boolean;
	loading?: boolean;
	maxSizeMB?: number;
	onUpload?: (file: File) => void;
	onRemove?: () => void;
	className?: string;
}

const DEFAULT_MAX_SIZE_MB = 5;

export function CoverImage({ url, editing, loading, maxSizeMB, onUpload, onRemove, className }: CoverImageProps) {
	const maxSizeBytes = (maxSizeMB ?? DEFAULT_MAX_SIZE_MB) * 1024 * 1024;

	if (loading) return <CoverImageSkeleton />;
	if (!url && !editing) return null;

	return (
		<div className={cn('relative overflow-hidden rounded-lg border border-border bg-muted', className)}>
			{url ? (
				<img src={url} alt="封面" loading="lazy" className="block h-36 w-full object-cover sm:h-44" />
			) : (
				<div className="flex h-36 w-full flex-col items-center justify-center gap-1.5 text-muted-foreground sm:h-44">
					<ImageIcon size={18} aria-hidden="true" />
					<span className="text-xs">暂无封面</span>
				</div>
			)}

			{editing ? (
				<div className="absolute right-2 bottom-2 flex items-center gap-2">
					<label className="inline-flex h-8 cursor-pointer items-center gap-1.5 rounded-md bg-background/95 px-2.5 text-xs font-medium text-foreground shadow-sm transition-colors hover:bg-background">
						<ImageIcon size={13} aria-hidden="true" />
						更换封面
						<input
							type="file"
							accept="image/*"
							className="absolute inset-0 cursor-pointer opacity-0"
							onChange={(event) => {
								const file = event.currentTarget.files?.[0];
								if (!file) return;
								if (file.size > maxSizeBytes) {
									toast.error(`封面大小不能超过 ${maxSizeMB ?? DEFAULT_MAX_SIZE_MB}MB`);
									event.currentTarget.value = '';
									return;
								}
								onUpload?.(file);
							}}
						/>
					</label>
					{url ? (
						<Button size="sm" variant="outline" className="bg-background/95" onClick={onRemove}>
							移除
						</Button>
					) : null}
				</div>
			) : null}
		</div>
	);
}
