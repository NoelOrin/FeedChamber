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

function readDomThemeMode(): ThemeMode {
  if (typeof document === 'undefined') return 'system'
  const mode = document.documentElement.getAttribute('data-theme-mode')
  if (mode === 'light' || mode === 'dark' || mode === 'system') return mode
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
  document.documentElement.setAttribute('data-theme-mode', mode)
}

export function initTheme() {
  applyTheme(getInitialTheme())
}

let _themeMode: (() => ThemeMode) | null = null
let _setThemeMode: ((v: ThemeMode) => void) | null = null

export function useTheme() {
  if (!_themeMode) {
    const initial = readDomThemeMode()
    const [mode, setMode] = createSignal<ThemeMode>(initial)
    _themeMode = mode
    _setThemeMode = setMode
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

  const cycleTheme = (event?: MouseEvent) => {
    const order: ThemeMode[] = ['light', 'dark', 'system']
    const next = order[(order.indexOf(themeMode()) + 1) % 3]

    const doApply = () => {
      setThemeMode(next)
      localStorage.setItem(STORAGE_KEY, next)
      applyTheme(next)
    }

    // View Transitions API 圆形扩散动画
    const supportsVT = typeof document !== 'undefined' && 'startViewTransition' in document
    if (!supportsVT || !event || window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      doApply()
      return
    }

    const x = event.clientX
    const y = event.clientY
    const endRadius = Math.hypot(
      Math.max(x, window.innerWidth - x),
      Math.max(y, window.innerHeight - y),
    )

    const transition = (document as any).startViewTransition(doApply)
    transition.ready.then(() => {
      document.documentElement.animate(
        { clipPath: [`circle(0px at ${x}px ${y}px)`, `circle(${endRadius}px at ${x}px ${y}px)`] },
        { duration: 400, easing: 'ease-in-out', pseudoElement: '::view-transition-new(root)' },
      )
    })
  }

  return { themeMode, themeTitle, toggleTheme: cycleTheme }
}


