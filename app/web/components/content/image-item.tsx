import { cn } from '~/lib/utils';

interface ImageItemProps {
	src: string;
	alt?: string;
	title?: string;
	className?: string;
}

/**
 * 正文图片块（旧 components/content/ImageItem.tsx 直译）。
 * React 19 会把组件里的 `<link rel="preload">` 提升到 <head>，preload 语义与旧版一致。
 */
export function ImageItem({ src, alt, title, className }: ImageItemProps) {
	return (
		<>
			<div
				className={cn(
					'flex max-h-[385px] select-none flex-col items-center justify-center overflow-hidden mb-2',
					className,
				)}
			>
				<img alt={alt ?? ''} src={src} loading="lazy" className="max-h-[385px] max-w-full object-contain" />
				{title ? <div className="-translate-y-1/4 text-xs opacity-80">{title}</div> : null}
			</div>
			<link rel="preload" href={src} as="image" />
		</>
	);
}

export default ImageItem;
