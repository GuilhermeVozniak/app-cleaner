// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'

vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(), EventsOff: vi.fn() }))
vi.mock('../../wailsjs/go/main/App', () => ({
  ListBackups: vi.fn().mockResolvedValue([]),
  RestoreBackup: vi.fn(),
  DeleteBackup: vi.fn(),
  CleanOldBackups: vi.fn().mockResolvedValue(0),
  GetBackupDetails: vi.fn().mockResolvedValue({ items: [], fromManifest: false, truncated: 0 }),
  GetConfig: vi.fn().mockResolvedValue(undefined),
  SaveConfig: vi.fn(),
  CheckFDA: vi.fn().mockResolvedValue(null),
}))

import { ListBackups, GetBackupDetails } from '../../wailsjs/go/main/App'
import Backups from './Backups'

const ListBackupsMock = ListBackups as unknown as ReturnType<typeof vi.fn>
const DetailsMock = GetBackupDetails as unknown as ReturnType<typeof vi.fn>

const session = { path: '/Users/me/Library/Application Support/AppCleaner/Backups/2026-07-18T00-00-00Z', date: '2026-07-18T00:00:00Z', size: 1024 }

beforeEach(() => {
  ListBackupsMock.mockClear()
  ListBackupsMock.mockResolvedValue([session])
  DetailsMock.mockReset()
  DetailsMock.mockResolvedValue({ items: [], fromManifest: false, truncated: 0 })
})
afterEach(() => cleanup())

describe('<Backups /> details', () => {
  it('expanding a session fetches details once and renders contracted paths with sizes', async () => {
    DetailsMock.mockResolvedValue({
      items: [{ path: '/Users/me/Library/Caches/Foo', name: 'Foo', size: 2048 }],
      fromManifest: true,
      truncated: 0,
    })
    render(<Backups />)
    const toggle = await screen.findByRole('button', { name: /toggle details/i })
    fireEvent.click(toggle)
    expect(await screen.findByText(/~\/Library\/Caches\/Foo \(2\.0 KB\)/)).toBeDefined()
    expect(DetailsMock).toHaveBeenCalledTimes(1)
    expect(DetailsMock).toHaveBeenCalledWith(session.path)
    // collapse + re-expand: cached, no second fetch
    fireEvent.click(toggle)
    fireEvent.click(toggle)
    expect(await screen.findByText(/~\/Library\/Caches\/Foo/)).toBeDefined()
    expect(DetailsMock).toHaveBeenCalledTimes(1)
  })

  it('shows the truncation tail', async () => {
    DetailsMock.mockResolvedValue({
      items: [{ path: '/Users/me/a', name: 'a', size: 1 }],
      fromManifest: false,
      truncated: 42,
    })
    render(<Backups />)
    fireEvent.click(await screen.findByRole('button', { name: /toggle details/i }))
    expect(await screen.findByText('…and 42 more files')).toBeDefined()
  })

  it('shows the empty state for sessions with no recorded details', async () => {
    render(<Backups />)
    fireEvent.click(await screen.findByRole('button', { name: /toggle details/i }))
    expect(await screen.findByText('No details recorded for this backup')).toBeDefined()
  })

  it('tolerates a null items payload (nil Go slice)', async () => {
    DetailsMock.mockResolvedValue({ items: null, fromManifest: false, truncated: 0 })
    render(<Backups />)
    fireEvent.click(await screen.findByRole('button', { name: /toggle details/i }))
    expect(await screen.findByText('No details recorded for this backup')).toBeDefined()
  })

  it('shows an error on fetch failure and retries on re-expand', async () => {
    DetailsMock.mockRejectedValueOnce(new Error('boom'))
    DetailsMock.mockResolvedValueOnce({ items: [{ path: '/Users/me/b', name: 'b', size: 1 }], fromManifest: true, truncated: 0 })
    render(<Backups />)
    const toggle = await screen.findByRole('button', { name: /toggle details/i })
    fireEvent.click(toggle)
    expect(await screen.findByText("Couldn't read backup details")).toBeDefined()
    fireEvent.click(toggle) // collapse
    fireEvent.click(toggle) // re-expand -> refetch
    expect(await screen.findByText(/~\/b \(1 B\)/)).toBeDefined()
    expect(DetailsMock).toHaveBeenCalledTimes(2)
  })
})
