import { describe, expect, it, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ProgressOverlay } from './ProgressOverlay'

describe('<ProgressOverlay />', () => {
  it('renders title, current item, and progress fraction', () => {
    render(<ProgressOverlay title="Cleaning…" current={2} total={4} itemName="/tmp/a.log" />)
    expect(screen.getByText('Cleaning…')).toBeInTheDocument()
    expect(screen.getByText('/tmp/a.log')).toBeInTheDocument()
    expect(screen.getByText('2 / 4')).toBeInTheDocument()
  })

  it('shows an ellipsis placeholder before the first item name arrives', () => {
    render(<ProgressOverlay title="Cleaning…" current={0} total={4} itemName="" />)
    expect(screen.getByText('…')).toBeInTheDocument()
  })

  it('only offers Cancel when a handler is provided, and forwards the click', () => {
    const onCancel = vi.fn()
    const { unmount } = render(
      <ProgressOverlay title="Cleaning…" current={1} total={4} itemName="x" onCancel={onCancel} />,
    )
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onCancel).toHaveBeenCalledTimes(1)
    unmount()

    render(<ProgressOverlay title="Cleaning…" current={1} total={4} itemName="x" />)
    expect(screen.queryByRole('button', { name: 'Cancel' })).toBeNull()
  })
})
