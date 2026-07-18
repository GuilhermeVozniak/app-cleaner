import { beforeEach, describe, expect, it } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import Sidebar from './Sidebar'
import { useUiStore } from '../stores/uiStore'

beforeEach(() => {
  useUiStore.setState({ view: 'smart-scan', activeCategoryId: undefined, fda: null, config: undefined })
})

describe('<Sidebar />', () => {
  it('renders every nav item', () => {
    render(<Sidebar />)
    for (const label of ['Smart Scan', 'Uninstaller', 'Maintenance', 'Backups', 'Settings']) {
      expect(screen.getByText(label)).toBeInTheDocument()
    }
  })

  it('treats the category detail view as part of Smart Scan for highlighting', () => {
    useUiStore.setState({ view: 'category', activeCategoryId: 'trash' })
    render(<Sidebar />)
    const btn = screen.getByText('Smart Scan').closest('button')!
    expect(btn.className).toContain('bg-blue-600')
  })

  it('does not highlight Smart Scan for an unrelated view', () => {
    useUiStore.setState({ view: 'settings' })
    render(<Sidebar />)
    const btn = screen.getByText('Smart Scan').closest('button')!
    expect(btn.className).not.toContain('bg-blue-600')
  })

  it('clicking a nav item calls setView', () => {
    render(<Sidebar />)
    fireEvent.click(screen.getByText('Settings'))
    expect(useUiStore.getState().view).toBe('settings')
  })

  it('shows the "Limited disk access" badge when fda is false, and routes to first-run on click', () => {
    useUiStore.setState({ fda: false })
    render(<Sidebar />)
    fireEvent.click(screen.getByText('Limited disk access'))
    expect(useUiStore.getState().view).toBe('first-run')
  })

  it('hides the disk-access badge when fda is true or unknown', () => {
    useUiStore.setState({ fda: true })
    const { rerender } = render(<Sidebar />)
    expect(screen.queryByText('Limited disk access')).toBeNull()
    useUiStore.setState({ fda: null })
    rerender(<Sidebar />)
    expect(screen.queryByText('Limited disk access')).toBeNull()
  })
})
