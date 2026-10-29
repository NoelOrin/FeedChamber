import { defineConfig } from 'tsup';

export default defineConfig({
	entry: {
		index: 'src/index.ts',
		'adapters/web/index': 'src/adapters/web/index.ts',
		'adapters/dashboard/index': 'src/adapters/dashboard/index.ts',
	},
	format: ['esm'],
	// 声明文件改由 `tsc -b tsconfig.build.json` 生成（增量，热构建 ~1s），
	// tsup 内置 dts 走 rollup 打包类型，单次约 20s 且没有缓存。
	dts: false,
	sourcemap: true,
	clean: false,
});
