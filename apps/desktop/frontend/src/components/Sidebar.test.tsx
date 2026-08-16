import { beforeEach, describe, expect, it } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import Sidebar from './Sidebar'
import { useUiStore } from '../stores/uiStore'

beforeEach(() => {
  useUiStore.setState({ view: 'dashboard', activeCategoryId: undefined, fda: null, config: undefined })
})

describe('<Sidebar /> (icon rail)', () => {
  it('renders a tile for every module plus Settings', () => {
    render(<Sidebar />)
    for (const label of [
      'Smart Care',
      'Cleanup',
      'Applications',
      'Performance',
      'Space Lens',
      'My Tools',
      'Backups',
      'Settings',
    ]) {
      expect(screen.getByRole('button', { name: label })).toBeInTheDocument()
    }
  })

  it('marks the active module tile with aria-current', () => {
    useUiStore.setState({ view: 'smart-scan' })
    render(<Sidebar />)
    expect(screen.getByRole('button', { name: 'Cleanup' })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('button', { name: 'Smart Care' })).not.toHaveAttribute('aria-current')
  })

  it('treats the category detail view as part of Cleanup for highlighting', () => {
    useUiStore.setState({ view: 'category', activeCategoryId: 'trash' })
    render(<Sidebar />)
    expect(screen.getByRole('button', { name: 'Cleanup' })).toHaveAttribute('aria-current', 'page')
  })

  it('treats the login-items view as part of Performance for highlighting', () => {
    useUiStore.setState({ view: 'login-items' })
    render(<Sidebar />)
    expect(screen.getByRole('button', { name: 'Performance' })).toHaveAttribute('aria-current', 'page')
  })

  it('clicking a tile calls setView', () => {
    render(<Sidebar />)
    fireEvent.click(screen.getByRole('button', { name: 'Settings' }))
    expect(useUiStore.getState().view).toBe('settings')
    fireEvent.click(screen.getByRole('button', { name: 'My Tools' }))
    expect(useUiStore.getState().view).toBe('my-tools')
  })

  it('shows the limited-disk-access tile when fda is false, and routes to first-run on click', () => {
    useUiStore.setState({ fda: false })
    render(<Sidebar />)
    fireEvent.click(screen.getByRole('button', { name: 'Limited disk access' }))
    expect(useUiStore.getState().view).toBe('first-run')
  })

  it('hides the disk-access tile when fda is true or unknown', () => {
    useUiStore.setState({ fda: true })
    const { rerender } = render(<Sidebar />)
    expect(screen.queryByRole('button', { name: 'Limited disk access' })).toBeNull()
    useUiStore.setState({ fda: null })
    rerender(<Sidebar />)
    expect(screen.queryByRole('button', { name: 'Limited disk access' })).toBeNull()
  })
})
