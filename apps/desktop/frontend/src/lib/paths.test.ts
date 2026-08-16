import { describe, expect, it } from 'vitest'
import { contractHome } from './paths'

describe('contractHome', () => {
  const home = '/Users/me'

  it('contracts the home dir itself to ~', () => {
    expect(contractHome('/Users/me', home)).toBe('~')
  })

  it('contracts home-prefixed paths to ~/…', () => {
    expect(contractHome('/Users/me/Documents/x.pdf', home)).toBe('~/Documents/x.pdf')
  })

  it('handles a home value with a trailing slash', () => {
    expect(contractHome('/Users/me/Library/Caches', '/Users/me/')).toBe('~/Library/Caches')
  })

  it('does not contract sibling users whose name shares the prefix', () => {
    expect(contractHome('/Users/melon/file', home)).toBe('/Users/melon/file')
  })

  it('passes non-home paths through untouched', () => {
    expect(contractHome('/opt/homebrew/bin', home)).toBe('/opt/homebrew/bin')
    expect(contractHome('docker:my-volume', home)).toBe('docker:my-volume')
  })

  it('passes everything through when home is unknown', () => {
    expect(contractHome('/Users/me/Documents', '')).toBe('/Users/me/Documents')
  })
})
