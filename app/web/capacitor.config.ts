import type { CapacitorConfig } from '@capacitor/cli';

try {
	require('dotenv').config({ path: '.env.mobile' });
} catch {
	// dotenv 未安装或 .env.mobile 不存在，使用环境变量/默认值
}

const serverUrl = process.env.MOBILE_SERVER_URL?.trim() || undefined;
const appId = process.env.CAPACITOR_APP_ID || 'com.orinforum.app';
const appName = process.env.CAPACITOR_APP_NAME || 'OrinForum';

const config: CapacitorConfig = {
	appId,
	appName,
	webDir: '.output/public',
	// 开发模式：指向 Vite dev server，支持 HMR
	// 生产模式：不设置 server，原生壳加载打包进 app 的静态资源
	...(serverUrl && {
		server: {
			url: serverUrl,
			cleartext: serverUrl.startsWith('http://'),
		},
	}),
	plugins: {
		SplashScreen: {
			launchAutoHide: true,
			androidScaleType: 'CENTER_CROP',
			backgroundColor: '#8c2de4',
		},
		Keyboard: {
			resize: 'body' as any,
			resizeOnFullScreen: true,
		},
		StatusBar: {
			style: 'DARK' as any,
			backgroundColor: '#8c2de4',
		},
	},
	ios: {
		contentInset: 'automatic',
		allowsLinkPreview: false,
	},
	android: {
		allowMixedContent: true,
		captureInput: true,
		webContentsDebuggingEnabled: process.env.NODE_ENV !== 'production',
	},
};

export default config;
