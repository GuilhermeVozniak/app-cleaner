import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

vi.mock('../../wailsjs/go/main/App', () => ({
  ListLoginItems: vi.fn(),
  RevealInFinder: vi.fn(),
}))

import { ListLoginItems, RevealInFinder } from '../../wailsjs/go/main/App'
import LoginItems, { groupLoginItems } from './LoginItems'
import type { LoginItem } from '../lib/types'

const ListMock = ListLoginItems as unknown as ReturnType<typeof vi.fn>
const RevealMock = RevealInFinder as unknown as ReturnType<typeof vi.fn>

function item(overrides: Partial<LoginItem>): LoginItem {
  return {
    label: 'com.example.agent',
    path: '/Users/me/Library/LaunchAgents/com.example.agent.plist',
    program: '/usr/local/bin/example',
    kind: 'user-agent',
    runAtLoad: false,
    programMissing: false,
    ...overrides,
  }
}

const fixtures: LoginItem[] = [
  item({ label: 'com.docker.helper', path: '/a/docker.plist', runAtLoad: true }),
  item({ label: 'com.gone.agent', path: '/a/gone.plist', program: '/opt/gone', programMissing: true }),
  item({ label: 'com.sys.daemon', path: '/d/sys.plist', kind: 'daemon' }),
]

beforeEach(() => {
  ListMock.mockReset().mockResolvedValue(fixtures)
  RevealMock.mockClear()
})

describe('groupLoginItems', () => {
  it('groups by kind in first-appearance order', () => {
    const groups = groupLoginItems([
      item({ kind: 'daemon', path: '/1' }),
      item({ kind: 'user-agent', path: '/2' }),
      item({ kind: 'daemon', path: '/3' }),
    ])
    expect(groups.map((g) => g.kind)).toEqual(['daemon', 'user-agent'])
    expect(groups[0].items.map((i) => i.path)).toEqual(['/1', '/3'])
  })

  it('returns an empty list for no items', () => {
    expect(groupLoginItems([])).toEqual([])
  })
})

describe('<LoginItems />', () => {
  it('lists agents and daemons under their kind headings', async () => {
    render(<LoginItems />)
    await waitFor(() => expect(screen.getByText('com.docker.helper')).toBeInTheDocument())
    expect(screen.getByText('Your agents')).toBeInTheDocument()
    expect(screen.getByText('System daemons')).toBeInTheDocument()
    expect(screen.getByText('/opt/gone')).toBeInTheDocument()
  })

  it('flags broken items and run-at-load items with badges', async () => {
    render(<LoginItems />)
    await waitFor(() => expect(screen.getByText('broken')).toBeInTheDocument())
    expect(screen.getByText('runs at load')).toBeInTheDocument()
  })

  it('Reveal calls the binding with the plist path', async () => {
    render(<LoginItems />)
    await waitFor(() => expect(screen.getByText('com.docker.helper')).toBeInTheDocument())
    fireEvent.click(screen.getAllByRole('button', { name: 'Reveal' })[0])
    expect(RevealMock).toHaveBeenCalledWith('/a/docker.plist')
  })

  it('Refresh re-fetches the list', async () => {
    render(<LoginItems />)
    await waitFor(() => expect(screen.getByText('com.docker.helper')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Refresh/ }))
    await waitFor(() => expect(ListMock).toHaveBeenCalledTimes(2))
  })

  it('shows the empty state when nothing launches automatically', async () => {
    ListMock.mockResolvedValue([])
    render(<LoginItems />)
    await waitFor(() => expect(screen.getByText('No login items')).toBeInTheDocument())
  })
})
