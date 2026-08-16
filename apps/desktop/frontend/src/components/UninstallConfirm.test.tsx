import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { UninstallConfirm } from './UninstallConfirm'
import type { AppInfo } from '../lib/types'

const MB = 1024 * 1024

const apps: AppInfo[] = [
  {
    name: 'Docker',
    path: '/Applications/Docker.app',
    bundleId: 'com.docker.docker',
    appSize: 500 * MB,
    relatedPaths: [{ path: '/Users/me/Library/Caches/com.docker.docker', size: 100 * MB }],
    totalSize: 600 * MB,
    running: false,
  },
]

const onCancel = vi.fn()
const onConfirm = vi.fn()

beforeEach(() => {
  onCancel.mockClear()
  onConfirm.mockClear()
})

describe('<UninstallConfirm />', () => {
  it('lists each app with its size and related leftovers', () => {
    render(<UninstallConfirm apps={apps} onCancel={onCancel} onConfirm={onConfirm} />)
    expect(screen.getByText('Uninstall applications')).toBeInTheDocument()
    expect(screen.getByText(/Docker/)).toBeInTheDocument()
    expect(screen.getAllByText(/600\.0 MB/).length).toBeGreaterThan(0) // app line + totals line
    expect(screen.getByText(/com\.docker\.docker/)).toBeInTheDocument()
    expect(screen.getByText(/will be freed/)).toBeInTheDocument()
  })

  it('Cancel forwards to onCancel', () => {
    render(<UninstallConfirm apps={apps} onCancel={onCancel} onConfirm={onConfirm} />)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onCancel).toHaveBeenCalledTimes(1)
  })

  it('confirms with dryRun=false by default', () => {
    render(<UninstallConfirm apps={apps} onCancel={onCancel} onConfirm={onConfirm} />)
    fireEvent.click(screen.getByRole('button', { name: 'Uninstall' }))
    expect(onConfirm).toHaveBeenCalledWith(false)
  })

  it('confirms with dryRun=true after ticking the dry-run checkbox', () => {
    render(<UninstallConfirm apps={apps} onCancel={onCancel} onConfirm={onConfirm} />)
    fireEvent.click(screen.getByRole('checkbox'))
    fireEvent.click(screen.getByRole('button', { name: 'Uninstall' }))
    expect(onConfirm).toHaveBeenCalledWith(true)
  })
})
