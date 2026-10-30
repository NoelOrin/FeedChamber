import { defineCollection } from 'astro:content';
import { docsLoader } from '@astrojs/starlight/loaders';
import { docsSchema } from '@astrojs/starlight/schema';

/**
 * 对外文档站的内容集合。
 *
 * 内容全部住在 `src/content/docs/`（本包内部），与仓库根的 `docs/` 物理隔离：
 * 后者是内部工程材料（plans / reviews / ADR / superpowers），既不该进对外导航，
 * 也不该被对外改写的节奏牵连。需要引用内部材料时从站内链回仓库。
 *
 * 多版本预留：现在是单版本（当前分支即唯一版本）。将来要并行版本时，
 * 把内容移入版本目录并接 `starlight-versions`，不需要改动本文件的结构。
 */
export const collections = {
	docs: defineCollection({
		loader: docsLoader(),
		schema: docsSchema(),
	}),
};
