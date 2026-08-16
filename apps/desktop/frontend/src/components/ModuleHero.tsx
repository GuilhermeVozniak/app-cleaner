import type { CSSProperties, ReactNode } from 'react'
import type { ModuleDef } from '../lib/modules'

interface ModuleHeroProps {
  module: ModuleDef
  /** The circular CTA (usually a ScanLens) rendered bottom-center. */
  cta: ReactNode
  /** Optional extra content between the feature list and the CTA. */
  children?: ReactNode
}

/**
 * CleanMyMac-style module hero: tinted wash from the top, big module icon,
 * title + description + feature bullets, circular CTA bottom-center. The
 * hue flows down through the --module indirection var.
 */
export function ModuleHero({ module: mod, cta, children }: ModuleHeroProps) {
  return (
    <div
      className="module-wash flex h-full flex-col items-center px-10 pt-14"
      style={{ '--module': mod.hue } as CSSProperties}
    >
      <div
        className="glass-1 module-glow flex h-24 w-24 items-center justify-center rounded-[28px]"
        aria-hidden
      >
        <mod.Icon size={44} style={{ color: 'var(--module)' }} />
      </div>
      <h1 className="mt-6 text-3xl font-bold text-ink">{mod.label}</h1>
      <p className="mt-2 max-w-md text-center text-sm text-ink-2">{mod.description}</p>
      {mod.features.length > 0 && (
        <ul className="mt-6 flex flex-wrap items-center justify-center gap-x-6 gap-y-2">
          {mod.features.map((f) => (
            <li key={f.label} className="flex items-center gap-2 text-sm text-ink">
              <f.Icon size={15} style={{ color: 'var(--module)' }} />
              {f.label}
            </li>
          ))}
        </ul>
      )}
      {children}
      <div className="mt-auto pb-12">{cta}</div>
    </div>
  )
}
