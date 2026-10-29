import path from 'node:path';
import { fileURLToPath } from 'node:url';
import tailwindcss from '@tailwindcss/vite';
import vinext from 'vinext';
import { defineConfig } from 'vite';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
	plugins: [tailwindcss(), vinext()],
	resolve: {
		alias: {
			'@': path.resolve(__dirname, '.'),
			'~': path.resolve(__dirname, '.'),
		},
	},
	server: {
		port: 3000,
		host: '0.0.0.0',
		proxy: {
			'/api/v1': {
				target: process.env.API_TARGET ?? 'http://localhost:8080',
				changeOrigin: true,
				timeout: 5000,
			},
		},
	},
});
