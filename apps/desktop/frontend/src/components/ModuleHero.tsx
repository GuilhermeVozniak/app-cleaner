import type { CSSProperties, ReactNode } from 'react'
import { ModuleIcon } from './ModuleIcon'
import type { ModuleDef } from '../lib/modules'

interface ModuleHeroProps {
  module: ModuleDef
  /** The orb, rendered bottom-centre. */
  cta: ReactNode
  /** Optional extra content under the feature list. */
  children?: ReactNode
}

/**
 * Module landing: big gem icon on the left, light display title, one-line
 * description and the feature list on the right, the orb bottom-centre. The
 * canvas behind it is already painted in the module's hue by the shell.
 */
export function ModuleHero({ module: mod, cta, children }: ModuleHeroProps) {
  return (
    <div
      className="materialize relative flex h-full flex-col overflow-hidden"
      style={{ '--module': mod.hue } as CSSProperties}
    >
      <div className="flex flex-1 items-center justify-center gap-20 px-16 pb-28">
        <ModuleIcon Icon={mod.Icon} size="hero" />
        <div className="max-w-[26rem]">
          <h1 className="text-display font-normal text-ink">{mod.label}</h1>
          <p className="mt-3 text-card text-ink-2">{mod.description}</p>
          {mod.features.length > 0 && (
            <ul className="mt-8 space-y-4">
              {mod.features.map((f) => (
                <li key={f.label} className="flex items-center gap-3 text-card font-semibold text-ink">
                  <ModuleIcon Icon={f.Icon} size="sm" />
                  {f.label}
                </li>
              ))}
            </ul>
          )}
          {children}
        </div>
      </div>
      <div className="absolute inset-x-0 bottom-0 flex justify-center pb-2">{cta}</div>
    </div>
  )
}

export default ModuleHero
