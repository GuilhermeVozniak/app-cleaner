import React from 'react'
import ReactDOM from 'react-dom/client'
import './style.css'

async function boot() {
  const params = new URLSearchParams(window.location.search)
  // Dev-only: `vite` + `?mock` renders the whole UI in a browser against a
  // fake Wails bridge (see src/dev/mock.ts). Installed before any store
  // module runs, since stores subscribe to runtime events at import time.
  if (import.meta.env.DEV && params.has('mock')) {
    const { installMock } = await import('./dev/mock')
    installMock(params)
  }
  const { default: App } = await import('./App')
  if (import.meta.env.DEV && params.get('view')) {
    const { useUiStore } = await import('./stores/uiStore')
    useUiStore.setState({ view: params.get('view') as never })
  }
  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <App />
    </React.StrictMode>,
  )
  if (import.meta.env.DEV && params.has('mock') && params.get('flow')) {
    const { runDevFlow } = await import('./dev/flows')
    await runDevFlow(params.get('flow') as string)
  }
}

void boot()
