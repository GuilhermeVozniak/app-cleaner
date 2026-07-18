// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { ActionBar } from './ActionBar'

afterEach(() => cleanup())

describe('<ActionBar />', () => {
  it('renders children inside a floating glass bar', () => {
    render(<ActionBar>controls</ActionBar>)
    const bar = screen.getByText('controls')
    expect(bar.closest('div')?.className).toContain('glass-2')
  })
})
