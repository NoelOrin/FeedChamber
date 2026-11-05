import { spawn } from 'node:child_process';
import { readFile } from 'node:fs/promises';
import { join } from 'node:path';

export interface DevOptions {
	dir: string;
	/** 真实站点地址。为空时用本地 mock 宿主。 */
	site?: string;
	/** 连真实站点时的凭证。 */
	token?: string;
	/** mock 模式下自动触发一次钩子，便于验证。 */
	hook?: string;
}

/**
 * 本地开发。
 *
 * 两种模式：
 * - 无 --site：启动内置 mock 宿主，插件连它，离线可跑；
 * - 有 --site：连真实站点（站点侧需已安装该插件并处于 running）。
 *
 * mock 宿主**刻意用同一个协议帧格式**（NDJSON + id 关联），否则「本地能跑、
 * 线上不行」是最难排查的一类问题。它与真宿主的差异只在：不校验 manifest、
 * 不鉴权、宿主 API 返回固定假数据。
 */
export async function devPlugin(opts: DevOptions): Promise<void> {
	const manifest = JSON.parse(await readFile(join(opts.dir, 'plugin.json'), 'utf8')) as {
		entry?: string;
		hooks?: string[];
		name: string;
	};

	if (opts.site) {
		console.error('连真实站点模式需要站点侧网关支持，v1 尚未提供。');
		console.error('当前请用不带 --site 的 mock 模式：fbp dev');
		process.exitCode = 1;
		return;
	}

	const entry = manifest.entry ?? 'index.js';
	const hooks = manifest.hooks ?? [];

	console.log(`[mock] 插件 ${manifest.name}，入口 ${entry}，钩子 ${hooks.join(', ') || '(无)'}`);

	const child = spawn(process.execPath, [join(opts.dir, entry)], {
		cwd: opts.dir,
		stdio: ['pipe', 'pipe', 'inherit'],
	});

	let nextId = 1;
	const pending = new Map<number, (frame: { result?: unknown; error?: { message: string } }) => void>();

	child.stdout.setEncoding('utf8');
	let buffer = '';
	child.stdout.on('data', (chunk: string) => {
		buffer += chunk;
		let i = buffer.indexOf('\n');
		while (i >= 0) {
			const line = buffer.slice(0, i);
			buffer = buffer.slice(i + 1);
			if (line.trim()) {
				handleLine(line);
			}
			i = buffer.indexOf('\n');
		}
	});

	function handleLine(line: string) {
		try {
			handleFrame(JSON.parse(line));
		} catch {
			console.error('[mock] 收到坏帧，已跳过:', line.slice(0, 80));
		}
	}

	function handleFrame(frame: {
		id?: number;
		method?: string;
		params?: Record<string, unknown>;
		result?: unknown;
		error?: { message: string };
	}) {
		// 有 method = 插件在调宿主 API。
		if (frame.method) {
			console.log(`[mock] 宿主 API 调用: ${frame.method}`, JSON.stringify(frame.params ?? {}));
			// mock 固定返回成功；真实宿主会校验 capability 与参数。
			child.stdin.write(JSON.stringify({ id: frame.id, result: { ok: true } }) + '\n');
			return;
		}
		// 无 method = 插件回包。
		const waiter = frame.id !== undefined ? pending.get(frame.id) : undefined;
		if (waiter && frame.id !== undefined) {
			pending.delete(frame.id);
			waiter(frame);
		}
	}

	function invokeHook(hook: string): Promise<void> {
		const id = nextId++;
		return new Promise((resolve, reject) => {
			pending.set(id, (frame) => {
				if (frame.error) {
					console.error(`[mock] 钩子 ${hook} 返回错误: ${frame.error.message}`);
					reject(new Error(frame.error.message));
					return;
				}
				console.log(`[mock] 钩子 ${hook} 返回:`, JSON.stringify(frame.result ?? null));
				resolve();
			});
			child.stdin.write(
				JSON.stringify({ id, method: `hook.${hook}`, params: { post_id: 'dev-post-1', title: 'mock post' } }) + '\n',
			);
		});
	}

	const first = opts.hook ?? hooks[0];
	if (first) {
		// 给插件一点时间建立 stdin 监听。
		setTimeout(() => {
			invokeHook(first).catch(() => {});
		}, 300);
	}

	const shutdown = () => {
		child.kill('SIGTERM');
		process.exit(0);
	};
	process.on('SIGINT', shutdown);
	process.on('SIGTERM', shutdown);
}
