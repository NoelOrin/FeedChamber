/**
 * 给 Markdown 正文里的站内绝对链接补上 base 前缀。
 *
 * 为什么需要：Starlight 会为侧栏与分页生成带 base 的链接，但**正文里手写的
 * `[x](/foo/)` 原样保留**。在 GitHub Pages 项目页（挂在 /<repo>/ 下）上，
 * 这些链接会跳到站点根而不是本仓库的产物目录 → 全站 404，且构建期不报错。
 *
 * 修在渲染管线而不是逐篇改 Markdown：新写的页面自动被覆盖，不会再犯。
 *
 * 跳过：外链（含协议相对 //）、页内锚点、以及由站点根提供的那批路径
 * （后端 API 示例、sitemap、robots 等 —— 它们不属于文档站）。
 */
const ROOT_PROVIDED = ['/api/v1', '/swagger', '/mcp', '/health', '/sitemap', '/robots.txt', '/latest.rss'];

export default function rehypeBaseLinks(options = {}) {
	const base = options.base ?? '/';
	// base 为 / 时（本地开发、自定义域名根）不需要任何改写。
	if (base === '/' || base === '') {
		return (tree) => tree;
	}
	const prefix = base.endsWith('/') ? base.slice(0, -1) : base;

	const rewrite = (url) => {
		if (typeof url !== 'string') return url;
		if (!url.startsWith('/')) return url; // 相对路径与外链
		if (url.startsWith('//')) return url; // 协议相对
		if (url.startsWith(prefix + '/') || url === prefix) return url; // 已带前缀
		if (ROOT_PROVIDED.some((p) => url === p || url.startsWith(p + '/'))) return url;
		return prefix + url;
	};

	const visit = (node) => {
		if (!node || typeof node !== 'object') return;
		if (node.type === 'element' && node.properties) {
			for (const key of ['href', 'src']) {
				if (key in node.properties) {
					node.properties[key] = rewrite(node.properties[key]);
				}
			}
		}
		if (Array.isArray(node.children)) node.children.forEach(visit);
	};

	return (tree) => {
		visit(tree);
		return tree;
	};
}
