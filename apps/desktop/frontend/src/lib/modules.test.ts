import { describe, expect, it } from 'vitest'
import { MODULES, TOOLS, filterTools, moduleForView } from './modules'

describe('MODULES registry', () => {
  it('has unique views and a hue + icon for every module', () => {
    const views = MODULES.map((m) => m.view)
    expect(new Set(views).size).toBe(views.length)
    for (const m of MODULES) {
      expect(m.hue).toMatch(/^var\(--color-module-/)
      expect(m.Icon).toBeTruthy()
      expect(m.label).toBeTruthy()
    }
  })
})

describe('moduleForView', () => {
  it('maps a module view to its own module', () => {
    expect(moduleForView('smart-scan')?.label).toBe('Cleanup')
    expect(moduleForView('dashboard')?.label).toBe('Smart Care')
    expect(moduleForView('backups')?.label).toBe('Backups')
  })

  it('maps nested views to their owning module', () => {
    expect(moduleForView('category')?.view).toBe('smart-scan')
    expect(moduleForView('login-items')?.view).toBe('maintenance')
  })

  it('returns undefined for views without a rail tile', () => {
    expect(moduleForView('settings')).toBeUndefined()
    expect(moduleForView('first-run')).toBeUndefined()
  })
})

describe('TOOLS registry', () => {
  it('every tool has a well-formed action', () => {
    for (const t of TOOLS) {
      if (t.action.kind === 'scan') expect(t.action.categoryId).toBeTruthy()
      else expect(t.action.view).toBeTruthy()
    }
  })

  it('surfaces the dedicated views as view tools', () => {
    const byId = Object.fromEntries(TOOLS.map((t) => [t.id, t]))
    expect(byId['space-lens'].action).toEqual({ kind: 'view', view: 'space-lens' })
    expect(byId['login-items'].action).toEqual({ kind: 'view', view: 'login-items' })
    expect(byId['maintenance'].action).toEqual({ kind: 'view', view: 'maintenance' })
  })
})

describe('filterTools', () => {
  it('returns everything for an empty or whitespace query', () => {
    expect(filterTools(TOOLS, '')).toEqual(TOOLS)
    expect(filterTools(TOOLS, '   ')).toEqual(TOOLS)
  })

  it('matches name case-insensitively and trims the query', () => {
    expect(filterTools(TOOLS, 'TRASH').map((t) => t.id)).toEqual(['trash-bins'])
    expect(filterTools(TOOLS, '  trash ').map((t) => t.id)).toEqual(['trash-bins'])
  })

  it('matches against descriptions too', () => {
    expect(filterTools(TOOLS, 'identical').map((t) => t.id)).toEqual(['duplicates'])
  })

  it('returns an empty list when nothing matches', () => {
    expect(filterTools(TOOLS, 'zzz-no-such-tool')).toEqual([])
  })
})
