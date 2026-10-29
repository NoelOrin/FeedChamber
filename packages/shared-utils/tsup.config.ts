import { defineConfig } from 'tsup';

export default defineConfig({
	entry: ['src/index.ts'],
	format: ['esm'],
	// 声明文件由 `tsc -b tsconfig.build.json` 生成，见 api-client/tsup.config.ts 注释
	dts: false,
	sourcemap: true,
	clean: false,
});
