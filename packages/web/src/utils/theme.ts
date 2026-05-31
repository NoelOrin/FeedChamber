import { createSignal } from 'solid-js'

type ThemeMode = 'light' | 'dark'

const STORAGE_KEY = 'theme'

function getSystemTheme(): ThemeMode {
  if (typeof window === 'undefined') return 'light'
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function getInitialTheme(): ThemeMode {
  if (typeof localStorage === 'undefined') return 'light'
  return (localStorage.getItem(STORAGE_KEY) as ThemeMode) || getSystemTheme()
}

function applyTheme(mode: ThemeMode) {
  if (typeof document === 'undefined') return
  document.documentElement.classList.toggle('dark', mode === 'dark')
}

export function initTheme() {
  applyTheme(getInitialTheme())
}

let _themeMode: (() => ThemeMode) | null = null
let _setThemeMode: ((v: ThemeMode) => void) | null = null

export function useTheme() {
  if (!_themeMode) {
    const [mode, setMode] = createSignal<ThemeMode>(getInitialTheme())
    _themeMode = mode
    _setThemeMode = setMode
  }

  const themeMode = _themeMode!
  const setThemeMode = _setThemeMode!

  return {
    themeMode,
    themeTitle: () => (themeMode() === 'dark' ? '切换亮色模式' : '切换暗色模式'),
    toggleTheme: () => {
      const next: ThemeMode = themeMode() === 'dark' ? 'light' : 'dark'
      setThemeMode(next)
      localStorage.setItem(STORAGE_KEY, next)
      applyTheme(next)
    },
  }
}

