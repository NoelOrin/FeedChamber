import { buildPlugin } from './commands/build';
import { devPlugin } from './commands/dev';
import { initPlugin } from './commands/init';

const USAGE = `fbp — Feed Chamber 插件开发 CLI

用法:
  fbp init <name>            生成插件工程（零第三方依赖）
  fbp dev                    启动本地 mock 宿主并拉起插件
  fbp dev --hook <name>      指定要触发的钩子（默认第一个）
  fbp build                  打包为 <name>-<version>.tgz

注意:
  v1 不提供 publish 子命令。插件分发走「仓库地址 + 人工审批」，
  由站长在后台登记来源并审批，不在 CLI 侧。
`;

function flag(args: string[], name: string): string | undefined {
	const i = args.indexOf(name);
	if (i < 0 || i + 1 >= args.length) return undefined;
	return args[i + 1];
}

async function main(): Promise<void> {
	const [cmd, ...rest] = process.argv.slice(2);

	switch (cmd) {
		case 'init': {
			const name = rest[0];
			if (!name) {
				console.error('用法: fbp init <name>');
				process.exitCode = 1;
				return;
			}
			const root = await initPlugin({ name, dir: process.cwd() });
			console.log(`已生成插件工程: ${root}`);
			console.log('下一步: cd ' + name + ' && fbp dev');
			return;
		}

		case 'dev':
			await devPlugin({
				dir: process.cwd(),
				site: flag(rest, '--site'),
				token: flag(rest, '--token'),
				hook: flag(rest, '--hook'),
			});
			return;

		case 'build':
			await buildPlugin({ dir: process.cwd() });
			return;

		case undefined:
		case '-h':
		case '--help':
		case 'help':
			console.log(USAGE);
			return;

		default:
			console.error(`未知子命令: ${cmd}\n`);
			console.error(USAGE);
			process.exitCode = 1;
	}
}

main().catch((err: unknown) => {
	console.error('fbp 出错:', err instanceof Error ? err.message : err);
	process.exitCode = 1;
});
