// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { Checkbox } from './checkbox'
import { Switch } from './switch'
import { Progress } from './progress'
import { Dialog, DialogContent, DialogTitle } from './dialog'

afterEach(() => cleanup())

describe('ui interactive primitives', () => {
  it('Checkbox toggles through aria-checked and fires onCheckedChange', () => {
    const onChange = vi.fn()
    render(<Checkbox aria-label="pick" checked={false} onCheckedChange={onChange} />)
    const box = screen.getByRole('checkbox', { name: 'pick' })
    expect(box.getAttribute('aria-checked')).toBe('false')
    fireEvent.click(box)
    expect(onChange).toHaveBeenCalledWith(true)
  })

  it('Switch reflects checked state', () => {
    const onChange = vi.fn()
    render(<Switch aria-label="toggle" checked onCheckedChange={onChange} />)
    expect(screen.getByRole('switch', { name: 'toggle' }).getAttribute('aria-checked')).toBe('true')
  })

  it('Progress exposes its value', () => {
    render(<Progress value={40} />)
    expect(screen.getByRole('progressbar')).toBeDefined()
  })

  it('Dialog renders content in a portal with a title', () => {
    render(
      <Dialog open>
        <DialogContent>
          <DialogTitle>Confirm things</DialogTitle>
          body
        </DialogContent>
      </Dialog>,
    )
    expect(screen.getByRole('dialog')).toBeDefined()
    expect(screen.getByText('Confirm things')).toBeDefined()
  })
})
