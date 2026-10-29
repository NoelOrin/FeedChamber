// Solid 版 `~/utils/sanitize.ts` 平移：DOMPurify 兜底 + rehype-sanitize schema，供 markdown-bridge 使用。
import DOMPurify from 'isomorphic-dompurify';
import { defaultSchema, type Options } from 'rehype-sanitize';

const SAFE_URI_RE = /^(?:(?:https?|mailto):|\/(?!\/)|[?#]|\.\.?\/|[^:/?#\\]+(?:[/?#]|$))/i;

export const markdownSanitizeSchema: Options = {
	...defaultSchema,
	protocols: {
		...defaultSchema.protocols,
		href: ['http', 'https', 'mailto'],
		src: ['http', 'https'],
	},
};

function decodeUriForCheck(uri: string): string {
	try {
		return decodeURIComponent(uri);
	} catch {
		return uri;
	}
}

export function sanitizeHtml(html: string): string {
	return DOMPurify.sanitize(html, {
		ALLOWED_URI_REGEXP: SAFE_URI_RE,
		FORCE_BODY: true,
		RETURN_DOM: false,
	});
}

DOMPurify.addHook('afterSanitizeAttributes', (node) => {
	// Check href/src attributes after DOMPurify's initial pass
	// to catch percent-encoded dangerous schemes that bypass the ALLOWED_URI_REGEXP
	for (const attr of ['href', 'src', 'xlink:href']) {
		const val = node.getAttribute(attr);
		if (val) {
			const decoded = decodeUriForCheck(val);
			if (/^\s*(?:javascript|vbscript|data|blob):/i.test(decoded)) {
				node.removeAttribute(attr);
			}
		}
	}
});
