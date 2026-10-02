import type { CSSProperties } from 'react'
import { MODULES, type ModuleCanvas } from '../lib/modules'

/**
 * Full-bleed module canvas behind the shell. The four gradient stops are
 * registered custom properties, so switching modules cross-fades the whole
 * window into the next hue instead of snapping.
 */
export function Backdrop({ canvas = MODULES[0].canvas }: { canvas?: ModuleCanvas }) {
  return (
    <div
      aria-hidden="true"
      className="module-canvas pointer-events-none fixed inset-0 z-0"
      style={
        {
          '--bg-1': canvas.bg1,
          '--bg-2': canvas.bg2,
          '--bg-3': canvas.bg3,
          '--bg-glow': canvas.glow,
        } as CSSProperties
      }
    />
  )
}

export default Backdrop
