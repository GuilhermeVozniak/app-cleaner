// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { ScanLens } from './ScanLens'

afterEach(() => cleanup())

describe('<ScanLens />', () => {
  it('idle: is a button named Smart Scan that fires onScan', () => {
    const onScan = vi.fn()
    render(<ScanLens state="idle" onScan={onScan} />)
    fireEvent.click(screen.getByRole('button', { name: /smart scan/i }))
    expect(onScan).toHaveBeenCalledTimes(1)
  })
  it('scanning: shows live progress and size, no button', () => {
    render(<ScanLens state="scanning" completed={3} total={16} totalSize={1073741824} onScan={() => {}} />)
    expect(screen.getByText('3/16')).toBeDefined()
    expect(screen.getByText('1.0 GB')).toBeDefined()
    expect(screen.queryByRole('button')).toBeNull()
  })
})
