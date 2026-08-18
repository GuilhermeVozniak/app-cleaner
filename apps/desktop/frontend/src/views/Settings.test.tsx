import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'

vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn().mockReturnValue(() => {}), EventsOff: vi.fn() }))
vi.mock('../../wailsjs/go/main/App', () => ({
  GetConfig: vi.fn(),
  SaveConfig: vi.fn(),
  CheckFDA: vi.fn().mockResolvedValue(null),
  CheckForUpdate: vi.fn().mockResolvedValue(null),
  GetVersion: vi.fn().mockResolvedValue('1.4.0'),
  DownloadUpdate: vi.fn(),
  InstallUpdate: vi.fn(),
  OpenDownloadedUpdate: vi.fn().mockResolvedValue(undefined),
}))

import { GetConfig, SaveConfig, DownloadUpdate, InstallUpdate, OpenDownloadedUpdate } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { Settings } from './Settings'
import { useUiStore } from '../stores/uiStore'
import type { Config } from '../lib/types'

const GetConfigMock = GetConfig as unknown as ReturnType<typeof vi.fn>
const SaveConfigMock = SaveConfig as unknown as ReturnType<typeof vi.fn>
const DownloadUpdateMock = DownloadUpdate as unknown as ReturnType<typeof vi.fn>
const InstallUpdateMock = InstallUpdate as unknown as ReturnType<typeof vi.fn>
const EventsOnMock = EventsOn as unknown as ReturnType<typeof vi.fn>

const CONFIG: Config = {
  downloadsDaysOld: 30,
  largeFilesMinSize: 524288000,
  backupByDefault: true,
  backupRetentionDays: 7,
  concurrency: 4,
  showRisky: false,
  keepLanguages: [],
  extraPaths: { nodeModules: [], projects: [] },
}

beforeEach(() => {
  useUiStore.setState({ config: undefined, update: undefined })
  GetConfigMock.mockReset()
  SaveConfigMock.mockReset()
  DownloadUpdateMock.mockReset()
  InstallUpdateMock.mockReset()
  EventsOnMock.mockClear()
  EventsOnMock.mockReturnValue(() => {})
})

describe('<Settings />', () => {
  it('shows a loading state until the config resolves, then renders the form', async () => {
    GetConfigMock.mockResolvedValue(CONFIG)
    render(<Settings />)
    expect(screen.getByText('Loading settings…')).toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: 'Settings' })).toBeInTheDocument()
  })

  it('renders the loaded config into the form fields', async () => {
    GetConfigMock.mockResolvedValue(CONFIG)
    render(<Settings />)
    const daysInput = (await screen.findByLabelText(/Downloads considered old after/)) as HTMLInputElement
    expect(daysInput.value).toBe('30')
    const concurrencyInput = screen.getByLabelText(/Scan concurrency/) as HTMLInputElement
    expect(concurrencyInput.value).toBe('4')
  })

  it('loads the config from the store directly when already set (no fetch)', () => {
    useUiStore.setState({ config: CONFIG })
    render(<Settings />)
    expect(screen.getByRole('heading', { name: 'Settings' })).toBeInTheDocument()
    expect(GetConfigMock).not.toHaveBeenCalled()
  })

  it('round-trips edits through save: clamps out-of-range input and calls SaveConfig', async () => {
    GetConfigMock.mockResolvedValue(CONFIG)
    SaveConfigMock.mockResolvedValue(undefined)
    render(<Settings />)
    const daysInput = (await screen.findByLabelText(/Downloads considered old after/)) as HTMLInputElement
    fireEvent.change(daysInput, { target: { value: '9999' } }) // out of the 1..365 bound

    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(SaveConfigMock).toHaveBeenCalledTimes(1))
    const saved = SaveConfigMock.mock.calls[0][0] as Config
    expect(saved.downloadsDaysOld).toBe(365) // clamped by fromForm before the call

    expect(await screen.findByText('Settings saved')).toBeInTheDocument()
    expect((screen.getByLabelText(/Downloads considered old after/) as HTMLInputElement).value).toBe('365')
  })

  it('shows a save-failed toast when SaveConfig rejects', async () => {
    GetConfigMock.mockResolvedValue(CONFIG)
    SaveConfigMock.mockRejectedValue(new Error('disk full'))
    render(<Settings />)
    await screen.findByLabelText(/Downloads considered old after/)
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    expect(await screen.findByText(/Save failed: Error: disk full/)).toBeInTheDocument()
  })

  it('toggles the backup-by-default and show-risky switches', async () => {
    GetConfigMock.mockResolvedValue(CONFIG)
    render(<Settings />)
    const backupSwitch = await screen.findByLabelText(/Back up by default/)
    const riskySwitch = screen.getByLabelText(/Show risky categories by default/)
    expect(backupSwitch.getAttribute('aria-checked')).toBe('true')
    expect(riskySwitch.getAttribute('aria-checked')).toBe('false')
    fireEvent.click(backupSwitch)
    fireEvent.click(riskySwitch)
    expect(backupSwitch.getAttribute('aria-checked')).toBe('false')
    expect(riskySwitch.getAttribute('aria-checked')).toBe('true')
  })

  it('edits the keepLanguages and extra scan-root text fields', async () => {
    GetConfigMock.mockResolvedValue(CONFIG)
    render(<Settings />)
    const langsInput = (await screen.findByLabelText(/Extra languages to keep/)) as HTMLInputElement
    fireEvent.change(langsInput, { target: { value: 'pt-BR, de' } })
    expect(langsInput.value).toBe('pt-BR, de')

    const nodeModulesInput = screen.getByLabelText(/Extra node_modules scan roots/) as HTMLTextAreaElement
    fireEvent.change(nodeModulesInput, { target: { value: '/a\n/b' } })
    expect(nodeModulesInput.value).toBe('/a\n/b')

    const projectsInput = screen.getByLabelText(/Extra project scan roots/) as HTMLTextAreaElement
    fireEvent.change(projectsInput, { target: { value: '/c' } })
    expect(projectsInput.value).toBe('/c')
  })

  it('shows the current version and an up-to-date note in the update card', async () => {
    GetConfigMock.mockResolvedValue(CONFIG)
    useUiStore.setState({
      update: { current: '1.4.0', latest: '1.4.0', available: false, url: 'https://x' },
    })
    render(<Settings />)
    expect(await screen.findByText(/Current version: 1\.4\.0/)).toBeInTheDocument()
    expect(screen.getByText(/up to date/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Get update' })).toBeNull()
  })

  it('Update now downloads with progress, installs, and reports relaunch', async () => {
    GetConfigMock.mockResolvedValue(CONFIG)
    useUiStore.setState({
      update: { current: '1.4.0', latest: '2.0.0', available: true, url: 'https://x' },
    })
    let resolveDownload: (v: string) => void = () => {}
    DownloadUpdateMock.mockImplementation(() => new Promise<string>((r) => (resolveDownload = r)))
    let resolveInstall: () => void = () => {}
    InstallUpdateMock.mockImplementation(() => new Promise<void>((r) => (resolveInstall = r)))

    render(<Settings />)
    expect(await screen.findByText(/New version 2\.0\.0 available/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Update now' }))

    // progress events stream in while the download promise is pending
    expect(DownloadUpdateMock).toHaveBeenCalledWith('2.0.0')
    const progressCall = EventsOnMock.mock.calls.find((c: unknown[]) => c[0] === 'update:progress')
    expect(progressCall).toBeDefined()
    const emit = progressCall![1] as (p: { done: number; total: number }) => void
    act(() => emit({ done: 50, total: 200 }))
    expect(await screen.findByText(/Downloading… 25%/)).toBeInTheDocument()

    await act(async () => {
      resolveDownload('/tmp/u.dmg')
    })
    expect(await screen.findByText('Installing…')).toBeInTheDocument()
    await act(async () => {
      resolveInstall()
    })
    // InstallUpdate only resolves without quitting if something odd happened;
    // normal success quits the app, so no further assertion on success UI.
    expect(InstallUpdateMock).toHaveBeenCalledTimes(1)
  })

  it('shows the manual fallback when the install fails', async () => {
    GetConfigMock.mockResolvedValue(CONFIG)
    useUiStore.setState({
      update: { current: '1.4.0', latest: '2.0.0', available: true, url: 'https://x' },
    })
    DownloadUpdateMock.mockResolvedValue('/tmp/u.dmg')
    InstallUpdateMock.mockRejectedValue(new Error('app is running translocated'))

    render(<Settings />)
    await screen.findByText(/New version 2\.0\.0 available/)
    fireEvent.click(screen.getByRole('button', { name: 'Update now' }))

    expect(await screen.findByText(/Couldn't install automatically/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Open downloaded update' }))
    expect(OpenDownloadedUpdate).toHaveBeenCalledTimes(1)
  })

  it('shows a download error and recovers to Update now', async () => {
    GetConfigMock.mockResolvedValue(CONFIG)
    useUiStore.setState({
      update: { current: '1.4.0', latest: '2.0.0', available: true, url: 'https://x' },
    })
    DownloadUpdateMock.mockRejectedValue(new Error('download returned 404'))

    render(<Settings />)
    await screen.findByText(/New version 2\.0\.0 available/)
    fireEvent.click(screen.getByRole('button', { name: 'Update now' }))

    expect(await screen.findByText(/Update failed:.*404/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Update now' })).toBeInTheDocument()
  })

  it('surfaces a failed update check without claiming up-to-date', async () => {
    GetConfigMock.mockResolvedValue(CONFIG)
    useUiStore.setState({
      update: { current: '1.4.0', latest: '', available: false, url: 'https://x', error: 'API 403' },
    })
    render(<Settings />)
    expect(await screen.findByText(/Could not check for updates: API 403/)).toBeInTheDocument()
    expect(screen.queryByText(/up to date/)).toBeNull()
  })

  it('edits the large-file threshold and backup retention number fields', async () => {
    GetConfigMock.mockResolvedValue(CONFIG)
    render(<Settings />)
    const largeFilesInput = (await screen.findByLabelText(/Large-file threshold/)) as HTMLInputElement
    fireEvent.change(largeFilesInput, { target: { value: '1000' } })
    expect(largeFilesInput.value).toBe('1000')

    const retentionInput = screen.getByLabelText(/Backup retention/) as HTMLInputElement
    fireEvent.change(retentionInput, { target: { value: '14' } })
    expect(retentionInput.value).toBe('14')
  })
})
