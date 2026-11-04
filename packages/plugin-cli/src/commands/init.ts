import { mkdir, writeFile } from 'node:fs/promises';
import { join } from 'node:path';

export interface InitOptions {
	/** 插件名，同时用作 plugin.json 的 name 与目录名。 */
	name: string;
	/** 生成到哪个目录（通常是 cwd）。 */
	dir: string;
}

/**
 * 生成一个最小可用插件工程。
 *
 * 产物**零第三方依赖**：协议客户端内联在 index.js 里，不 import 任何
 * `@feedchamber/*` 包。这是刻意的——插件打包产物不该把 CLI 的版本
 * 绑进插件生命周期，否则 CLI 停更会让存量插件失效。
 */
export async function initPlugin({ name, dir }: InitOptions): Promise<string> {
	const root = join(dir, name);
	await mkdir(root, { recursive: true });

	await writeFile(
		join(root, 'plugin.json'),
		JSON.stringify(
			{
				name,
				version: '0.1.0',
				protocol_version: '1',
				description: '',
				author: '',
				entry: 'index.js',
				hooks: ['post.created'],
				capabilities: [],
			},
			null,
			'\t',
		) + '\n',
	);

	await writeFile(join(root, 'index.js'), PLUGIN_TEMPLATE(name));
	await writeFile(join(root, 'README.md'), README_TEMPLATE(name));

	return root;
}

/** 插件入口模板。协议读写全在这里，作者只写 handler。 */
function PLUGIN_TEMPLATE(name: string): string {
	return `// ${name} — Feed Chamber 插件入口。
//
// 站点以 \`node index.js\` 拉起本进程，通过 stdin/stdout 上的 NDJSON 帧通信：
//   { "id": 7, "method": "hook.post.created", "params": { ... } }   宿主 → 插件
//   { "id": 7, "result": { ... } }                                   插件 → 宿主
//   { "id": 1000001, "method": "api.posts.addTag", "params": { ... } }  插件 → 宿主
//
// 不要 import 任何 @feedchamber/* 包：插件产物必须零框架依赖。

const pending = new Map();
let nextId = 1_000_000;

/** 调用宿主 API 并等待结果。方法名与参数见站点 docs/design/plugin-hooks.md。 */
function call(method, params) {
	const id = nextId++;
	return new Promise((resolve, reject) => {
		pending.set(id, { resolve, reject });
		write({ id, method, params });
	});
}

function write(frame) {
	process.stdout.write(JSON.stringify(frame) + '\\n');
}

function reply(id, result) {
	write({ id, result });
}

function replyError(id, message) {
	write({ id, error: { message } });
}

// ── 钩子实现 ────────────────────────────────────────────────
// 键必须是 plugin.json 里 hooks 声明过的钩子名。

const hooks = {
	'post.created': async (params) => {
		// params 是事件的 payload（如 { post_id, title, space_id }）。
		// 需要写数据时调宿主 API，例如：
		//   await call('api.posts.addTag', { post_id: params.post_id, tag: 'auto' });
		return { ok: true, saw: params.post_id ?? null };
	},
};

// ── 帧分发 ──────────────────────────────────────────────────

let buffer = '';

process.stdin.setEncoding('utf8');
process.stdin.on('data', (chunk) => {
	buffer += chunk;
	let i;
	while ((i = buffer.indexOf('\\n')) >= 0) {
		const line = buffer.slice(0, i);
		buffer = buffer.slice(i + 1);
		if (!line.trim()) continue;
		dispatch(line);
	}
});

function dispatch(line) {
	let frame;
	try {
		frame = JSON.parse(line);
	} catch {
		return; // 坏帧跳过，不拆连接
	}

	// 无 method = 响应。按 id 回到等待者。
	if (!frame.method) {
		const waiter = pending.get(frame.id);
		if (waiter) {
			pending.delete(frame.id);
			if (frame.error) waiter.reject(new Error(frame.error.message));
			else waiter.resolve(frame.result);
		}
		return;
	}

	// 有 method = 宿主请求。
	const hookName = frame.method.startsWith('hook.') ? frame.method.slice(5) : null;
	const handler = hookName ? hooks[hookName] : null;

	if (!handler) {
		replyError(frame.id, 'unknown method: ' + frame.method);
		return;
	}

	Promise.resolve()
		.then(() => handler(frame.params ?? {}))
		.then((result) => reply(frame.id, result))
		.catch((err) => replyError(frame.id, err.message));
}

process.on('SIGTERM', () => process.exit(0));
`;
}

function README_TEMPLATE(name: string): string {
	return `# ${name}

Feed Chamber 插件。

## 结构

- \`plugin.json\` — 插件清单：协议版本、订阅的钩子、申请的能力。
- \`index.js\` — 入口。\`hooks\` 对象里每个键是钩子名，值是处理函数。

## 开发

\`\`\`bash
fbp dev          # 启动本地 mock 宿主
fbp dev --site https://your-forum.example --token <token>   # 连真实站点
\`\`\`

## 钩子

在 \`plugin.json\` 的 \`hooks\` 里声明，然后实现 \`hooks['<名>']\`。当前支持：

\`post.created\` \`post.closed\` \`comment.created\` \`user.registered\`
\`user.followed\` \`report.created\` \`moderation.decided\` \`cron.daily\`

## 能力

在 \`plugin.json\` 的 \`capabilities\` 里申请，宿主在安装时展示给管理员。
未申请的能力调用会被拒绝。

可用能力：\`posts:read\` \`posts:write\` \`comments:read\` \`comments:write\`
\`users:read\` \`notifications:send\` \`kv:read\` \`kv:write\` \`slots:write\`

## 打包

\`\`\`bash
fbp build        # 产出 <name>-<version>.tgz
\`\`\`
`;
}
