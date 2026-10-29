import type { Extensions } from '@tiptap/core';
import Image from '@tiptap/extension-image';
import Link from '@tiptap/extension-link';
import Placeholder from '@tiptap/extension-placeholder';
import TaskItem from '@tiptap/extension-task-item';
import TaskList from '@tiptap/extension-task-list';
import Underline from '@tiptap/extension-underline';
import StarterKit from '@tiptap/starter-kit';

import { isSafeHref, isSafeSrc } from './safe-url';

/**
 * Solid 版 extensions.ts 平移。唯一差异：Tiptap 3.31 的 StarterKit 默认内置 Link/Underline，
 * 显式置 false 后仍由下方独立实例接管（保持旧版配置项一致，避免同名扩展重复注册）。
 */
const SafeImage = Image.extend({
	parseHTML() {
		return [
			{
				tag: 'img[src]:not([src^="data:"])',
				getAttrs: (element) => {
					const src = element instanceof HTMLElement ? element.getAttribute('src') : null;
					return isSafeSrc(src) ? null : false;
				},
			},
		];
	},
	addAttributes() {
		return {
			...this.parent?.(),
			src: {
				default: null,
				renderHTML: (attrs) => {
					const src = attrs.src as string;
					if (!isSafeSrc(src)) return {};
					return { src };
				},
			},
			alt: { default: null },
			title: { default: null },
			width: { default: null },
			height: { default: null },
		};
	},
});

export function getExtensions(placeholder: () => string): Extensions {
	return [
		StarterKit.configure({
			heading: { levels: [1, 2, 3] },
			link: false,
			underline: false,
		}),
		Underline,
		Placeholder.configure({
			placeholder: () => placeholder() || 'Write something...',
		}),
		Link.configure({
			openOnClick: false,
			HTMLAttributes: {
				class: 'text-blue-500 hover:underline cursor-pointer',
				rel: 'nofollow noopener',
				target: '_blank',
			},
			isAllowedUri: (href) => isSafeHref(href),
			shouldAutoLink: (href) => isSafeHref(href),
			validate: (href) => isSafeHref(href),
		}),
		SafeImage.configure({
			inline: false,
			allowBase64: false,
		}),
		TaskList,
		TaskItem.configure({
			nested: true,
		}),
	];
}
