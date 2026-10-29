import type * as React from 'react';

import { cn } from '~/lib/utils';

/*
 * 正文排版：shadcn typography 的克制版本。
 * 层级只靠字号 / 字重 / 上下留白，不用左侧粗边、虚线底线这类印刷装饰。
 */
const PROSE_CLASS = cn(
	'text-[15px] leading-7 break-words text-foreground',

	'[&_p]:mb-3.5 [&_dl]:mb-3.5 [&_ul]:mb-3.5 [&_ol]:mb-3.5 [&_blockquote]:mb-3.5 [&_pre]:mb-3.5 [&_table]:mb-3.5',

	'[&_h1]:mt-7 [&_h2]:mt-7 [&_h3]:mt-6 [&_h4]:mt-5',
	'[&_h1]:mb-3 [&_h2]:mb-3 [&_h3]:mb-2.5 [&_h4]:mb-2',
	'[&_h1]:text-[26px] [&_h1]:leading-tight [&_h1]:font-semibold',
	'[&_h2]:border-b [&_h2]:border-border [&_h2]:pb-1.5 [&_h2]:text-[20px] [&_h2]:leading-snug [&_h2]:font-semibold',
	'[&_h3]:text-[15px] [&_h3]:leading-snug [&_h3]:font-semibold',
	'[&_h4]:text-[15px] [&_h4]:leading-snug [&_h4]:font-semibold',

	'[&_strong]:font-semibold',
	'[&_a]:font-medium [&_a]:text-brand-text [&_a]:underline [&_a]:underline-offset-4 [&_a]:decoration-brand-text/40 hover:[&_a]:decoration-brand-text',

	'[&_ul]:list-disc [&_ul]:pl-5 [&_ol]:list-decimal [&_ol]:pl-5',
	'[&_li]:my-1 [&_li]:pl-0.5 [&_li]:marker:text-muted-foreground',
	'[&_li_p]:mb-1',

	'[&_blockquote]:border-l-2 [&_blockquote]:border-border [&_blockquote]:pl-4 [&_blockquote]:text-muted-foreground',

	'[&_code]:rounded [&_code]:bg-muted [&_code]:px-1.5 [&_code]:py-0.5 [&_code]:font-mono [&_code]:text-[0.85em]',
	'[&_pre]:overflow-x-auto [&_pre]:rounded-md [&_pre]:border [&_pre]:border-border [&_pre]:bg-muted [&_pre]:p-3 [&_pre]:text-[13px] [&_pre]:leading-relaxed',
	'[&_pre_code]:bg-transparent [&_pre_code]:p-0 [&_pre_code]:text-[0.95em]',

	'[&_img]:my-3 [&_img]:block [&_img]:max-w-full [&_img]:rounded-md [&_img]:border [&_img]:border-border [&_img]:object-contain',
	'[&_hr]:my-7 [&_hr]:border-0 [&_hr]:border-t [&_hr]:border-border',

	'[&_table]:w-full [&_table]:border-collapse [&_table]:text-[13px]',
	'[&_th]:border [&_th]:border-border [&_th]:bg-muted [&_th]:px-3 [&_th]:py-2 [&_th]:text-left [&_th]:font-medium',
	'[&_td]:border [&_td]:border-border [&_td]:px-3 [&_td]:py-2 [&_td]:text-left',
	'[&_td]:align-top',
);

export function Prose({ className, ...props }: React.ComponentProps<'div'>) {
	return <div data-slot="prose" className={cn(PROSE_CLASS, className)} {...props} />;
}
