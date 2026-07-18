/**
 * Static aurora canvas behind the shell: three large pre-blurred radial
 * gradients on the canvas color. Deliberately NO backdrop-filter and NO
 * animation — the glass layers above pay the blur cost, not the backdrop.
 */
export function Backdrop() {
  return (
    <div aria-hidden="true" className="pointer-events-none fixed inset-0 z-0">
      <div
        className="absolute inset-0"
        style={{
          background: [
            `radial-gradient(42% 38% at 18% 12%, color-mix(in srgb, var(--color-aurora-sky) calc(var(--aurora-alpha) * 100%), transparent), transparent 70%)`,
            `radial-gradient(46% 42% at 85% 20%, color-mix(in srgb, var(--color-aurora-violet) calc(var(--aurora-alpha) * 100%), transparent), transparent 70%)`,
            `radial-gradient(50% 46% at 50% 95%, color-mix(in srgb, var(--color-aurora-mint) calc(var(--aurora-alpha) * 100%), transparent), transparent 70%)`,
          ].join(', '),
        }}
      />
    </div>
  )
}

export default Backdrop
