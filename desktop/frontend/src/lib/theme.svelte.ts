// Light / dark / follow-system appearance. The choice is a per-machine convenience, so
// it lives in localStorage; app.css reads it from the data-theme attribute.
export type Theme = 'system' | 'light' | 'dark'

const key = 'pare.theme'

function stored(): Theme {
  try {
    const value = localStorage.getItem(key)
    if (value === 'light' || value === 'dark') return value
  } catch {}
  return 'system'
}

export const appearance = $state({ theme: stored() })

function apply(theme: Theme) {
  if (theme === 'system') document.documentElement.removeAttribute('data-theme')
  else document.documentElement.setAttribute('data-theme', theme)
}

export function setTheme(theme: Theme) {
  appearance.theme = theme
  apply(theme)
  try {
    localStorage.setItem(key, theme)
  } catch {}
}

export function initTheme() {
  apply(appearance.theme)
}
