import { useEffect } from 'react';
import { create } from 'zustand';

import { toast } from './toast';

type ThemeMode = 'light' | 'dark' | 'system';
type ResolvedTheme = 'light' | 'dark';

const STORAGE_KEY = 'theme';
const ORDER: ThemeMode[] = ['light', 'dark', 'system'];

function systemTheme(): ResolvedTheme {
	if (typeof window === 'undefined') return 'light';
	return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function resolve(mode: ThemeMode): ResolvedTheme {
	return mode === 'system' ? systemTheme() : mode;
}

function applyTheme(mode: ThemeMode): void {
	if (typeof document === 'undefined') return;
	const resolved = resolve(mode);
	document.documentElement.classList.toggle('dark', resolved === 'dark');
	document.documentElement.setAttribute('data-theme', resolved === 'dark' ? 'orindark' : 'orinlight');
	document.documentElement.setAttribute('data-theme-mode', mode);
}

function readDom(): ThemeMode {
	if (typeof document === 'undefined') return 'system';
	const mode = document.documentElement.getAttribute('data-theme-mode');
	return mode === 'light' || mode === 'dark' || mode === 'system' ? mode : 'system';
}

/* 首帧固定 'system' 与 SSR 输出一致，挂载后再从 DOM 读回真实值，避免水合不一致。
   `resolved` 是解析后的明暗（system 时跟随系统），antd 之类需要具体明暗的消费方直接读它。 */
const useThemeStore = create<{
	mode: ThemeMode;
	resolved: ResolvedTheme;
	set: (mode: ThemeMode) => void;
	setResolved: (resolved: ResolvedTheme) => void;
}>((set) => ({
	mode: 'system',
	resolved: 'light',
	set: (mode) => set({ mode, resolved: resolve(mode) }),
	setResolved: (resolved) => set({ resolved }),
}));

function syncResolved(): void {
	if (typeof document === 'undefined') return;
	useThemeStore.getState().setResolved(document.documentElement.classList.contains('dark') ? 'dark' : 'light');
}

function setTheme(mode: ThemeMode): void {
	if (mode !== 'light' && mode !== 'dark' && mode !== 'system') return;
	localStorage.setItem(STORAGE_KEY, mode);
	applyTheme(mode);
	useThemeStore.getState().set(mode);
}

/** 服务端偏好（themeService.getPreference）落到本地时调用。 */
export function syncThemeFromServer(mode: unknown): void {
	if (mode !== 'light' && mode !== 'dark' && mode !== 'system') return;
	if (localStorage.getItem(STORAGE_KEY) === mode) return;
	setTheme(mode);
}

export function useTheme() {
	const mode = useThemeStore((state) => state.mode);
	const resolved = useThemeStore((state) => state.resolved);

	// 与首屏内联脚本保持同源：DOM 才是真值，挂载后读回，并只注册一次系统偏好监听
	useEffect(() => {
		const domMode = readDom();
		if (useThemeStore.getState().mode !== domMode) useThemeStore.getState().set(domMode);
		syncResolved();
		bindSystemListener();
	}, []);

	const title = { light: '切换暗色模式', dark: '跟随系统', system: '切换亮色模式' }[mode];

	function cycle(_event?: MouseEvent, onChange?: (mode: ThemeMode) => void) {
		// 主题切换直接换色：不做圆形扩散动画（装饰性动效一律去掉）
		const next = ORDER[(ORDER.indexOf(mode) + 1) % ORDER.length];
		setTheme(next);
		onChange?.(next);
		/*
		 * 提示统一在这里发：前台 ThemeToggle 与后台 AdminShell 的三个切换入口共用 cycle()，
		 * 放在调用方会漏掉后台。文案读切换后的 <html> 真实明暗，zustand 的 resolved 要等下一次渲染。
		 */
		const isDark = document.documentElement.classList.contains('dark');
		toast.success(
			next === 'light'
				? '已切换到亮色模式'
				: next === 'dark'
					? '已切换到暗色模式'
					: `已切换为跟随系统（当前${isDark ? '暗色' : '亮色'}）`,
		);
	}

	return { mode, isDark: resolved === 'dark', setMode: setTheme, title, cycle };
}

let listenerBound = false;

function bindSystemListener() {
	if (listenerBound) return;
	listenerBound = true;
	window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
		if (useThemeStore.getState().mode === 'system') {
			applyTheme('system');
			syncResolved();
		}
	});
}
