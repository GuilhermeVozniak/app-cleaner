// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { Button } from './button'
import { Card } from './card'
import { Badge } from './badge'
import { Separator } from './separator'
import { Input } from './input'

afterEach(() => cleanup())

describe('ui static primitives', () => {
  it('Button renders every variant, fires onClick, and disables', () => {
    const onClick = vi.fn()
    render(<Button onClick={onClick}>Go</Button>)
    fireEvent.click(screen.getByRole('button', { name: 'Go' }))
    expect(onClick).toHaveBeenCalledTimes(1)
    for (const variant of ['primary', 'destructive', 'ghost', 'glass'] as const) {
      const { unmount } = render(<Button variant={variant}>{variant}</Button>)
      expect(screen.getByRole('button', { name: variant })).toBeDefined()
      unmount()
    }
    render(<Button disabled>Off</Button>)
    expect((screen.getByRole('button', { name: 'Off' }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('Card applies glass elevation classes', () => {
    const { container } = render(<Card elevation={2}>x</Card>)
    expect((container.firstElementChild as HTMLElement).className).toContain('glass-2')
    const { container: c1 } = render(<Card>y</Card>)
    expect((c1.firstElementChild as HTMLElement).className).toContain('glass-1')
  })

  it('Badge variants render their label', () => {
    for (const variant of ['safe', 'moderate', 'risky', 'neutral'] as const) {
      const { unmount } = render(<Badge variant={variant}>{variant}</Badge>)
      expect(screen.getByText(variant)).toBeDefined()
      unmount()
    }
  })

  it('Separator is decorative; Input forwards value + onChange', () => {
    const { container } = render(<Separator />)
    expect((container.firstElementChild as HTMLElement).getAttribute('role')).toBe('none')
    const onChange = vi.fn()
    render(<Input aria-label="days" value="7" onChange={onChange} />)
    fireEvent.change(screen.getByLabelText('days'), { target: { value: '9' } })
    expect(onChange).toHaveBeenCalled()
  })
})
