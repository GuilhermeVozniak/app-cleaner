import { beforeEach, describe, expect, it } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import MyTools, { runTool } from './MyTools'
import { TOOLS } from '../lib/modules'
import { useScanStore } from '../stores/scanStore'
import { useUiStore } from '../stores/uiStore'

const tool = (id: string) => TOOLS.find((t) => t.id === id)!

beforeEach(() => {
  useScanStore.getState().reset()
  useUiStore.setState({ view: 'my-tools', fda: null, config: undefined })
})

describe('runTool', () => {
  it('view tools just switch the view', () => {
    runTool(tool('login-items'))
    expect(useUiStore.getState().view).toBe('login-items')
    expect(useScanStore.getState().status).toBe('idle')
  })

  it('scan tools land on Cleanup and start a single-category scan', async () => {
    runTool(tool('trash-bins'))
    expect(useUiStore.getState().view).toBe('smart-scan')
    await waitFor(() => expect(useScanStore.getState().status).toBe('scanning'))
    expect(useScanStore.getState().scanIds).toEqual(['trash'])
  })

  it('does not restart a scan that is already running', async () => {
    useScanStore.setState({ status: 'scanning', scanIds: ['downloads'] })
    runTool(tool('trash-bins'))
    expect(useUiStore.getState().view).toBe('smart-scan')
    await Promise.resolve()
    expect(useScanStore.getState().scanIds).toEqual(['downloads'])
  })
})

describe('<MyTools />', () => {
  it('renders every tool card', () => {
    render(<MyTools />)
    for (const t of TOOLS) {
      expect(screen.getByText(t.name)).toBeInTheDocument()
    }
  })

  it('search narrows the grid', () => {
    render(<MyTools />)
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search tools' }), {
      target: { value: 'mail' },
    })
    expect(screen.getByText('Mail Attachments')).toBeInTheDocument()
    expect(screen.queryByText('Trash Bins')).toBeNull()
  })

  it('shows an empty message when nothing matches', () => {
    render(<MyTools />)
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search tools' }), {
      target: { value: 'zzz' },
    })
    expect(screen.getByText(/No tools match/)).toBeInTheDocument()
  })

  it('an Open card routes to its view', () => {
    render(<MyTools />)
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search tools' }), {
      target: { value: 'login' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Open' }))
    expect(useUiStore.getState().view).toBe('login-items')
  })
})
