import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

vi.mock('../../wailsjs/go/main/App', () => ({
  BuildSpaceLens: vi.fn(),
  GetHome: vi.fn().mockResolvedValue(null),
}))

import { BuildSpaceLens } from '../../wailsjs/go/main/App'
import SpaceLens, { nodeAt } from './SpaceLens'
import type { SpaceLensNode } from '../lib/types'

const BuildMock = BuildSpaceLens as unknown as ReturnType<typeof vi.fn>

const file = (name: string, path: string, size: number): SpaceLensNode => ({
  name,
  path,
  size,
  isDir: false,
})

const tree: SpaceLensNode = {
  name: 'me',
  path: '/u/me',
  size: 300,
  isDir: true,
  children: [
    {
      name: 'Documents',
      path: '/u/me/Documents',
      size: 200,
      isDir: true,
      children: [file('a.pdf', '/u/me/Documents/a.pdf', 150)],
      truncated: 3,
    },
    file('movie.mp4', '/u/me/movie.mp4', 100),
  ],
}

beforeEach(() => {
  BuildMock.mockReset()
})

describe('nodeAt', () => {
  it('returns the root for an empty trail', () => {
    expect(nodeAt(tree, [])).toBe(tree)
  })

  it('walks the trail into nested dirs', () => {
    expect(nodeAt(tree, ['/u/me/Documents']).name).toBe('Documents')
  })

  it('falls back to the deepest found node when a trail entry is missing', () => {
    expect(nodeAt(tree, ['/nope'])).toBe(tree)
    expect(nodeAt(tree, ['/u/me/Documents', '/nope']).name).toBe('Documents')
  })
})

describe('<SpaceLens />', () => {
  it('idle hero scans on click and renders the size map', async () => {
    BuildMock.mockResolvedValue(tree)
    render(<SpaceLens />)
    fireEvent.click(screen.getByRole('button', { name: 'Scan' }))
    await waitFor(() => expect(screen.getByText('Documents')).toBeInTheDocument())
    expect(screen.getByText('movie.mp4')).toBeInTheDocument()
    expect(screen.getByText('/u/me')).toBeInTheDocument()
    expect(screen.getByText('300 B')).toBeInTheDocument()
  })

  it('drills into a directory and back out again', async () => {
    BuildMock.mockResolvedValue(tree)
    render(<SpaceLens />)
    fireEvent.click(screen.getByRole('button', { name: 'Scan' }))
    await waitFor(() => expect(screen.getByText('Documents')).toBeInTheDocument())

    fireEvent.click(screen.getByText('Documents'))
    expect(screen.getByText('a.pdf')).toBeInTheDocument()
    expect(screen.getByText('…and 3 smaller items')).toBeInTheDocument()
    expect(screen.queryByText('movie.mp4')).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: 'Back' }))
    expect(screen.getByText('movie.mp4')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Back' })).toBeNull()
  })

  it('surfaces build errors on the hero and allows retry', async () => {
    BuildMock.mockRejectedValueOnce(new Error('not under home'))
    render(<SpaceLens />)
    fireEvent.click(screen.getByRole('button', { name: 'Scan' }))
    await waitFor(() => expect(screen.getByText(/not under home/)).toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'Scan' })).toBeInTheDocument()
  })
})
