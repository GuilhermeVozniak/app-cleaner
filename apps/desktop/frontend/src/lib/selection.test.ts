import { describe, expect, it } from 'vitest'
import {
  DEFAULT_DIR_LIMIT,
  EXPAND_INCREMENT,
  bumpExpand,
  invertSelection,
  togglePath,
  toggleDirectory,
} from './selection'

describe('bumpExpand', () => {
  it('starts a directory at the default limit + one increment', () => {
    expect(bumpExpand({}, 'dirA')).toEqual({ dirA: DEFAULT_DIR_LIMIT + EXPAND_INCREMENT })
  })

  it('bumps an existing limit and never mutates the input', () => {
    const before = { dirA: 15 }
    const after = bumpExpand(before, 'dirA')
    expect(after).toEqual({ dirA: 25 })
    expect(before).toEqual({ dirA: 15 })
  })
})

describe('togglePath', () => {
  const all = ['/a', '/b', '/c']

  it("materializes 'all' into a Set minus the toggled path", () => {
    expect(togglePath(all, 'all', '/b')).toEqual(new Set(['/a', '/c']))
  })

  it('adds to an undefined selection', () => {
    expect(togglePath(all, undefined, '/a')).toEqual(new Set(['/a']))
  })

  it('removes an already-selected path', () => {
    expect(togglePath(all, new Set(['/a']), '/a')).toEqual(new Set())
  })
})

describe('invertSelection', () => {
  const all = ['/a', '/b']

  it("'all' inverts to nothing", () => {
    expect(invertSelection(all, 'all')).toEqual(new Set())
  })

  it('a partial Set inverts to its complement', () => {
    expect(invertSelection(all, new Set(['/a']))).toEqual(new Set(['/b']))
  })

  it('undefined inverts to everything', () => {
    expect(invertSelection(all, undefined)).toEqual(new Set(all))
  })
})

describe('toggleDirectory', () => {
  const all = ['/d/1', '/d/2', '/d/3', '/e/1']

  it('deselects when every visible file is already selected', () => {
    const sel = new Set(['/d/1', '/d/2', '/e/1'])
    expect(toggleDirectory(all, sel, ['/d/1', '/d/2'])).toEqual(new Set(['/e/1']))
  })

  it('selects all visible files when only some are selected', () => {
    const sel = new Set(['/d/1'])
    expect(toggleDirectory(all, sel, ['/d/1', '/d/2'])).toEqual(new Set(['/d/1', '/d/2']))
  })

  it("materializes 'all' and only touches the visible paths", () => {
    expect(toggleDirectory(all, 'all', ['/d/1', '/d/2'])).toEqual(new Set(['/d/3', '/e/1']))
  })

  it('leaves hidden files behind an expand-hint untouched', () => {
    const sel = new Set(['/d/3'])
    expect(toggleDirectory(all, sel, ['/d/1'])).toEqual(new Set(['/d/1', '/d/3']))
  })
})
