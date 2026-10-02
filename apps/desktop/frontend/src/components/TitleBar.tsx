import type { CSSProperties } from 'react'

/**
 * The 44px strip under the hidden-inset macOS title bar. It is the window's
 * drag handle and names the current module in the centre, the way a native
 * toolbar title would. Views place <StartOver /> into its left side.
 */
export function TitleBar({ title }: { title: string }) {
  return (
    <div
      className="pointer-events-auto fixed inset-x-0 top-0 z-30 flex h-11 items-center justify-center"
      style={{ '--wails-draggable': 'drag' } as CSSProperties}
    >
      <span className="text-body font-semibold text-ink-2">{title}</span>
    </div>
  )
}

export default TitleBar
