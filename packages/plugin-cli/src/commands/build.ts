import { execFile } from 'node:child_process';
import { readFile, stat } from 'node:fs/promises';
import { join } from 'node:path';
import { promisify } from 'node:util';

const exec = promisify(execFile);

export interface BuildOptions {
	dir: string;
}

/**
 * 打包插件为 tar.gz。
 *
 * 只打 `plugin.json` + 入口 + 源码目录，**排除** `node_modules` 与测试文件：
 * 依赖由站点在安装时按 package.json 自行安装（安装流程有超时与清理窗口），
 * 把 node_modules 塞进包里会让产物随依赖膨胀，且与站点的依赖树冲突。
 */
export async function buildPlugin({ dir }: BuildOptions): Promise<void> {
	const manifestPath = join(dir, 'plugin.json');
	const raw = await readFile(manifestPath, 'utf8').catch(() => null);
	if (!raw) {
		console.error('找不到 plugin.json。请在插件目录里运行，或先 fbp init。');
		process.exitCode = 1;
		return;
	}

	const manifest = JSON.parse(raw) as { name?: string; version?: string; entry?: string };
	if (!manifest.name || !manifest.version) {
		console.error('plugin.json 缺少 name 或 version。');
		process.exitCode = 1;
		return;
	}

	const entry = manifest.entry ?? 'index.js';
	const entryStat = await stat(join(dir, entry)).catch(() => null);
	if (!entryStat) {
		console.error(`入口文件 ${entry} 不存在（plugin.json 的 entry 指向它）。`);
		process.exitCode = 1;
		return;
	}

	const out = `${manifest.name}-${manifest.version}.tgz`;

	// 用 tar 排除 node_modules 与常见开发产物。--exclude 是 tar 的标准选项，
	// 各平台 GNU tar / bsdtar 都支持。
	const args = ['-czf', out, '--exclude=node_modules', '--exclude=*.tgz', '--exclude=.git', '--exclude=*.test.js', '.'];

	try {
		await exec('tar', args, { cwd: dir });
		console.log(`已生成 ${join(dir, out)}`);
	} catch (err) {
		console.error('打包失败:', err instanceof Error ? err.message : err);
		process.exitCode = 1;
	}
}
