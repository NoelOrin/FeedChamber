// Solid 版 `~/utils/safe-url.ts` 原样平移：URL 白名单是 XSS 防线，逻辑一字未动。
const HREF_PROTOCOLS = new Set(['http:', 'https:', 'mailto:']);
const SRC_PROTOCOLS = new Set(['http:', 'https:']);
const SCHEME_RE = /^[a-z][a-z0-9+.-]*:/i;
const CONTROL_CHARS_RE = /[\p{Cc}\s]+/u;

function normalizeUrl(url: string | undefined | null): string {
	const trimmed = (url ?? '').trim();
	if (CONTROL_CHARS_RE.test(trimmed)) return '';
	// Decode percent-encoded sequences to catch obfuscated schemes (e.g. %6a%61%76%61%73%63%72%69%70%74:)
	try {
		const decoded = decodeURIComponent(trimmed);
		if (CONTROL_CHARS_RE.test(decoded)) return '';
		return decoded;
	} catch {
		// Invalid percent-encoding — reject
		return '';
	}
}

function isSafeRelativeUrl(url: string): boolean {
	if (!url) return false;
	if (url.startsWith('//')) return false;
	if (url.startsWith('\\')) return false;
	return true;
}

function isAllowedUrl(url: string | undefined | null, protocols: Set<string>, allowRelative: boolean): boolean {
	const normalized = normalizeUrl(url);
	if (!normalized) return false;
	if (!SCHEME_RE.test(normalized)) return allowRelative && isSafeRelativeUrl(normalized);

	try {
		return protocols.has(new URL(normalized).protocol.toLowerCase());
	} catch {
		return false;
	}
}

export function isSafeHref(url: string | undefined | null): boolean {
	return isAllowedUrl(url, HREF_PROTOCOLS, true);
}

export function isSafeSrc(url: string | undefined | null): boolean {
	return isAllowedUrl(url, SRC_PROTOCOLS, true);
}
