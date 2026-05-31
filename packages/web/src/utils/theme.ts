import { createSignal, onCleanup } from 'solid-js'

export type ThemeMode = 'light' | 'dark' | 'system'
type ResolvedTheme = 'light' | 'dark'

const STORAGE_KEY = 'theme'

function getSystemTheme(): ResolvedTheme {
  if (typeof window === 'undefined') return 'light'
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function getInitialTheme(): ThemeMode {
  if (typeof localStorage === 'undefined') return 'system'
  const saved = localStorage.getItem(STORAGE_KEY) as ThemeMode | null
  if (saved === 'light' || saved === 'dark' || saved === 'system') return saved
  return 'system'
}

function resolveTheme(mode: ThemeMode): ResolvedTheme {
  return mode === 'system' ? getSystemTheme() : mode
}

function applyTheme(mode: ThemeMode) {
  if (typeof document === 'undefined') return
  const resolved = resolveTheme(mode)
  document.documentElement.classList.toggle('dark', resolved === 'dark')
  document.documentElement.setAttribute('data-theme', resolved)
}

export function initTheme() {
  applyTheme(getInitialTheme())
}

let _themeMode: (() => ThemeMode) | null = null
let _setThemeMode: ((v: ThemeMode) => void) | null = null

export function useTheme() {
  if (!_themeMode) {
    const initial = getInitialTheme()
    const [mode, setMode] = createSignal<ThemeMode>(initial)
    _themeMode = mode
    _setThemeMode = setMode
    applyTheme(initial)
  }

  const themeMode = _themeMode!
  const setThemeMode = _setThemeMode!

  // 监听系统主题变化，仅在 system 模式下生效
  if (typeof window !== 'undefined') {
    const mql = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = () => {
      if (themeMode() === 'system') applyTheme('system')
    }
    mql.addEventListener('change', handler)
    onCleanup(() => mql.removeEventListener('change', handler))
  }

  const themeTitle = () => {
    const titles: Record<ThemeMode, string> = {
      light: '切换暗色模式',
      dark: '跟随系统',
      system: '切换亮色模式',
    }
    return titles[themeMode()]
  }

  const cycleTheme = () => {
    const order: ThemeMode[] = ['light', 'dark', 'system']
    const next = order[(order.indexOf(themeMode()) + 1) % 3]
    setThemeMode(next)
    localStorage.setItem(STORAGE_KEY, next)
    applyTheme(next)
  }

  return { themeMode, themeTitle, toggleTheme: cycleTheme }
}

