import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { waitFor } from '@testing-library/react'

// main.tsx unconditionally renders <App/>, which calls CheckFDA on mount;
// stub it so the entry-point smoke test doesn't depend on the real wailsjs
// bridge's default (null) resolution racing the assertion below.
vi.mock('../wailsjs/go/main/App', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../wailsjs/go/main/App')>()
  return { ...actual, CheckFDA: vi.fn().mockResolvedValue(true) }
})

let root: HTMLDivElement

beforeEach(() => {
  root = document.createElement('div')
  root.id = 'root'
  document.body.appendChild(root)
})

afterEach(() => {
  document.body.removeChild(root)
})

describe('main entry point', () => {
  it('mounts the app into #root without throwing', async () => {
    await import('./main')
    await waitFor(() => expect(root.children.length).toBeGreaterThan(0))
  })
})
