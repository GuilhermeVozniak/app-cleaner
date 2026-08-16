import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import SizeBar from './SizeBar'

function valueNow(): string | null {
  return screen.getByRole('progressbar').getAttribute('aria-valuenow')
}

describe('<SizeBar />', () => {
  it('renders the size proportional to maxSize', () => {
    render(<SizeBar size={50} maxSize={100} />)
    expect(valueNow()).toBe('50')
  })

  it('keeps a minimum 2% sliver for tiny non-zero sizes', () => {
    render(<SizeBar size={1} maxSize={1000} />)
    expect(valueNow()).toBe('2')
  })

  it('renders 0 for a zero size', () => {
    render(<SizeBar size={0} maxSize={1000} />)
    expect(valueNow()).toBe('0')
  })

  it('renders 0 when maxSize is 0 (no divide-by-zero)', () => {
    render(<SizeBar size={10} maxSize={0} />)
    expect(valueNow()).toBe('0')
  })

  it('caps at 100 when size exceeds maxSize', () => {
    render(<SizeBar size={2000} maxSize={1000} />)
    expect(valueNow()).toBe('100')
  })
})
