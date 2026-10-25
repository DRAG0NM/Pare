import { mount } from 'svelte'
import App from './App.svelte'
import '@fontsource/doto/latin-800.css'
import '@fontsource/space-mono/latin-400.css'
import '@fontsource/space-mono/latin-700.css'
import './app.css'
import { initTheme } from './lib/theme.svelte'
import { initLocale } from './lib/i18n.svelte'

initTheme()
initLocale()

export default mount(App, { target: document.getElementById('app')! })
