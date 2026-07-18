// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest'
import { render, cleanup } from '@testing-library/react'
import { Backdrop } from './Backdrop'

afterEach(() => cleanup())

describe('<Backdrop />', () => {
  it('renders a fixed, pointer-transparent, aria-hidden layer', () => {
    const { container } = render(<Backdrop />)
    const el = container.firstElementChild as HTMLElement
    expect(el.getAttribute('aria-hidden')).toBe('true')
    expect(el.className).toContain('pointer-events-none')
    expect(el.className).toContain('fixed')
  })
})
