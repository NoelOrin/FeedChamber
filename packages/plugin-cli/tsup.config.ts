import { defineConfig } from 'tsup';

export default defineConfig({
	entry: { index: 'src/index.ts' },
	format: ['esm'],
	banner: { js: '#!/usr/bin/env node' },
	target: 'node20',
	clean: true,
});
