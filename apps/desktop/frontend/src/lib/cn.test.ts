import { describe, it, expect } from 'vitest'
import { cn } from './cn'

describe('cn', () => {
  it('merges conditional classes and resolves tailwind conflicts', () => {
    expect(cn('p-2', false && 'hidden', 'p-4')).toBe('p-4')
    expect(cn('text-ink', undefined, 'font-mono')).toBe('text-ink font-mono')
  })
})
