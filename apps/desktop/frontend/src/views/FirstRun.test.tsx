import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

vi.mock('../../wailsjs/go/main/App', () => ({
  OpenFDASettings: vi.fn(),
  CheckFDA: vi.fn(),
}))

import { OpenFDASettings, CheckFDA } from '../../wailsjs/go/main/App'
import FirstRun from './FirstRun'
import { useUiStore } from '../stores/uiStore'

const OpenFDASettingsMock = OpenFDASettings as unknown as ReturnType<typeof vi.fn>
const CheckFDAMock = CheckFDA as unknown as ReturnType<typeof vi.fn>

beforeEach(() => {
  useUiStore.setState({ view: 'first-run', fda: null, config: undefined })
  OpenFDASettingsMock.mockClear()
  CheckFDAMock.mockReset()
  CheckFDAMock.mockResolvedValue(null)
})

describe('<FirstRun />', () => {
  it('renders the FDA prompt', () => {
    render(<FirstRun />)
    expect(screen.getByText('Grant Full Disk Access')).toBeInTheDocument()
    expect(screen.getByText('Trash')).toBeInTheDocument()
    expect(screen.getByText('Safari cache')).toBeInTheDocument()
    expect(screen.getByText('Mail attachments')).toBeInTheDocument()
  })

  it('"Open System Settings" calls OpenFDASettings', () => {
    render(<FirstRun />)
    fireEvent.click(screen.getByText('Open System Settings'))
    expect(OpenFDASettingsMock).toHaveBeenCalledTimes(1)
  })

  it('"Re-check" calls CheckFDA; a grant switches the view to smart-scan', async () => {
    CheckFDAMock.mockResolvedValueOnce(true)
    render(<FirstRun />)
    fireEvent.click(screen.getByText('Re-check'))
    expect(CheckFDAMock).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(useUiStore.getState().fda).toBe(true))
    expect(useUiStore.getState().view).toBe('smart-scan')
  })

  it('"Re-check" leaves the view on first-run when still denied', async () => {
    CheckFDAMock.mockResolvedValueOnce(false)
    render(<FirstRun />)
    fireEvent.click(screen.getByText('Re-check'))
    await waitFor(() => expect(useUiStore.getState().fda).toBe(false))
    expect(useUiStore.getState().view).toBe('first-run')
  })

  it('"Continue without" switches to smart-scan directly', () => {
    render(<FirstRun />)
    fireEvent.click(screen.getByText('Continue without'))
    expect(useUiStore.getState().view).toBe('smart-scan')
  })

  it('re-checks FDA on window focus', async () => {
    CheckFDAMock.mockResolvedValueOnce(true)
    render(<FirstRun />)
    fireEvent(window, new Event('focus'))
    await waitFor(() => expect(useUiStore.getState().fda).toBe(true))
  })
})
