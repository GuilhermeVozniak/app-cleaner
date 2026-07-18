import { beforeEach, describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { CleanFlow } from './CleanFlow'
import { useCleanStore } from '../stores/cleanStore'
import { useScanStore } from '../stores/scanStore'
import { useUiStore } from '../stores/uiStore'

beforeEach(() => {
  useCleanStore.getState().reset()
  useScanStore.getState().reset()
  useUiStore.setState({ config: undefined })
})

describe('<CleanFlow />', () => {
  it('renders nothing when idle', () => {
    const { container } = render(<CleanFlow />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders the confirm modal while confirming', () => {
    useCleanStore.getState().openConfirm()
    render(<CleanFlow />)
    expect(screen.getByText('Confirm clean')).toBeInTheDocument()
  })

  it('renders a progress overlay while cleaning, prefixing the item name during backup', () => {
    useCleanStore.setState({
      status: 'cleaning',
      backingUp: true,
      progress: { current: 1, total: 4, categoryId: '', itemName: 'photo.jpg' },
    })
    render(<CleanFlow />)
    expect(screen.getByText('Backing up: photo.jpg')).toBeInTheDocument()
  })

  it('renders a "Cleaning…" title when not a dry run and not backing up', () => {
    useCleanStore.setState({
      status: 'cleaning',
      lastDryRun: false,
      backingUp: false,
      progress: { current: 1, total: 4, categoryId: 'trash', itemName: 'a.zip' },
    })
    render(<CleanFlow />)
    expect(screen.getByText('Cleaning…')).toBeInTheDocument()
    expect(screen.getByText('a.zip')).toBeInTheDocument()
  })

  it('renders a "Dry run…" title when lastDryRun is set', () => {
    useCleanStore.setState({ status: 'cleaning', lastDryRun: true })
    render(<CleanFlow />)
    expect(screen.getByText('Dry run…')).toBeInTheDocument()
  })

  it('renders the results panel when done', () => {
    useCleanStore.setState({
      status: 'done',
      summary: { results: [], totalFreedSpace: 500, totalCleanedItems: 1, totalErrors: 0 },
    })
    render(<CleanFlow />)
    expect(screen.getByText(/freed/)).toBeInTheDocument()
  })
})
