'use client';

import { lazy, Suspense, useEffect, useState } from 'react';

import { CODE_PRE_CLASS } from '~/components/content/code-surface';
import { Check, Copy } from '~/lib/icons';
import { cn } from '~/lib/utils';

/* 高亮器是懒加载块：旧 Solid 版用 `createResource(() => getHighlighter())`，
 迁移计划 §3.3 指定改为 lazy + Suspense（首帧渲染未高亮的 <pre>，与旧版 fallback 一致）。 */
const CodeHighlight = lazy(() => import('~/components/content/code-highlight'));

interface CodeBlockProps {
	code: string;
	language?: string;
	filename?: string;
	className?: string;
}

/** 高亮块未就绪时的降级态，与旧 CodeBlock 的 fallback 分支同构（纯文本 <pre>）。 */
function PlainCode({ code, language }: { code: string; language?: string }) {
	return (
		<pre className={CODE_PRE_CLASS}>
			<code className={language ? `language-${language}` : undefined}>{code}</code>
		</pre>
	);
}

export function CodeBlock({ code, language, filename, className }: CodeBlockProps) {
	const [copied, setCopied] = useState(false);

	// 旧 onCleanup(() => clearTimeout(copyTimer)) → effect cleanup
	useEffect(() => {
		if (!copied) return;
		const timer = setTimeout(() => setCopied(false), 2000);
		return () => clearTimeout(timer);
	}, [copied]);

	const handleCopy = async () => {
		try {
			await navigator.clipboard.writeText(code);
			setCopied(true);
		} catch (err) {
			console.error('Failed to copy:', err);
		}
	};

	return (
		<div className={cn('group relative my-4', className)}>
			{filename ? (
				<div className="absolute top-0 left-0 z-10 rounded-tr-md rounded-bl-md border border-border bg-secondary px-3 py-1 text-xs text-secondary-foreground">
					{filename}
				</div>
			) : null}
			<button
				type="button"
				onClick={handleCopy}
				title={copied ? '已复制' : '复制'}
				aria-label={copied ? '已复制' : '复制代码'}
				className={cn(
					'absolute top-2 right-2 z-10 inline-flex cursor-pointer items-center justify-center border border-border',
					'bg-card p-1.5 text-muted-foreground opacity-0 transition-[opacity,color,border-color] duration-[180ms]',
					'group-hover:opacity-100 group-focus-within:opacity-100 hover:border-foreground/14 hover:text-foreground',
				)}
			>
				{copied ? <Check className="size-4" aria-hidden="true" /> : <Copy className="size-4" aria-hidden="true" />}
			</button>
			<Suspense fallback={<PlainCode code={code} language={language} />}>
				<CodeHighlight code={code} language={language ?? 'text'} />
			</Suspense>
		</div>
	);
}

export default CodeBlock;
