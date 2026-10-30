#!/usr/bin/env node
/**
 * 把仓库根的 openapi.json 同步到文档站的 public/。
 *
 * 不手放副本：那份规格由 `make swag` 从 handler 注解生成，手放一份必然漂移。
 * 产物进 .gitignore（构建时生成），单一真相源仍是仓库根那份。
 */
import { copyFileSync, existsSync, mkdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(here, '..', '..', '..');
const source = join(repoRoot, 'openapi.json');
const targetDir = join(here, '..', 'public');
const target = join(targetDir, 'openapi.json');

if (!existsSync(source)) {
	console.error(`✗ 找不到 ${source}\n  先在 app/server 跑 make swag 生成 openapi.json。`);
	process.exit(1);
}

mkdirSync(targetDir, { recursive: true });
copyFileSync(source, target);
console.log(`✓ openapi.json → ${target}`);
