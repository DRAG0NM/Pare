import { mount } from 'svelte'
import App from './App.svelte'
import './app.css'
import { initTheme } from './lib/theme.svelte'
import { initLocale } from './lib/i18n.svelte'

initTheme()
initLocale()

export default mount(App, { target: document.getElementById('app')! })
