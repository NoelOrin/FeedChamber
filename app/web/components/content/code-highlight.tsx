'use client';

import { useMemo } from 'react';

import { CODE_PRE_CLASS } from '~/components/content/code-surface';
import { type CodeToken, highlight, type TokenKind } from '~/components/content/code-tokenizer';

interface CodeHighlightProps {
	code: string;
	language?: string;
}

/* 单色高亮：关键词最重、字面量次之、注释最轻，全部由前景色透明度分层。 */
const TOKEN_CLASS: Record<TokenKind, string> = {
	comment: 'italic text-[color-mix(in_srgb,var(--foreground)_44%,transparent)]',
	string: 'text-[color-mix(in_srgb,var(--foreground)_74%,transparent)]',
	number: 'text-[color-mix(in_srgb,var(--foreground)_74%,transparent)] tabular-nums',
	keyword: 'font-semibold text-foreground',
	literal: 'italic text-[color-mix(in_srgb,var(--foreground)_62%,transparent)]',
	plain: '',
};

function tokenNode(token: CodeToken, index: number) {
	const cls = TOKEN_CLASS[token.kind];
	// token 序列是高亮结果的线性切片，位置即身份，没有可复用的 id
	if (!cls) return <span key={`t-${index}`}>{token.text}</span>;
	return (
		<span key={`t-${index}`} className={cls}>
			{token.text}
		</span>
	);
}

/**
 * 懒加载的高亮层：`CodeBlock` 通过 `lazy()` 拿到本文件，未就绪时先渲染未高亮的 `<pre>`。
 * 旧 Solid 版把 shiki 的 `codeToHtml` 结果直接 innerHTML；这里改为分词后交给 React 渲染，
 * 正文的 HTML 注入点因此只剩 markdown-renderer 一处。
 */
export default function CodeHighlight({ code, language }: CodeHighlightProps) {
	const tokens = useMemo(() => highlight(code, language), [code, language]);

	return (
		<pre className={CODE_PRE_CLASS}>
			<code className={`language-${language ?? 'text'}`}>{tokens.map(tokenNode)}</code>
		</pre>
	);
}
