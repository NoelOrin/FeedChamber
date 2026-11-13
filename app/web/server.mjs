import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { startProdServer } from 'vinext/server/prod-server';

/**
 * 生产启动入口（容器里跑的是它的打包产物）。
 *
 * 为什么不用 `vinext start`：那个命令会在启动时 `import()` vinext 整个 dist
 * 树（服务器、构建器、配置加载器全都要在磁盘上），所以镜像必须带上 vinext
 * 及其全部依赖。这里只调 `startProdServer` —— 它是真正跑请求的那部分，
 * 依赖闭合、且不碰构建期模块。Dockerfile 用 `bun build` 把它连同 vinext
 * 的运行时打包成单文件，运行的镜像里就只剩产物 dist/ 和 react/react-dom
 * （`dist/server/vinext-externals.json` 也只列这两个）。
 *
 * 本地开发仍用 `pnpm dev`（vinext dev），这个文件只在生产容器里用。
 */
const outDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), 'dist');

await startProdServer({
	port: Number(process.env.PORT ?? 3000),
	host: process.env.HOSTNAME ?? '0.0.0.0',
	outDir,
});
