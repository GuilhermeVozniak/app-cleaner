import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import CategoryCard from './CategoryCard'
import type { Category, ScanResult } from '../lib/types'

const category: Category = {
  id: 'trash',
  name: 'Trash',
  group: 'System Junk',
  description: '',
  safetyLevel: 'safe',
}

function result(overrides: Partial<ScanResult> = {}): ScanResult {
  return { category, items: [], totalSize: 2048, ...overrides }
}

const onToggle = vi.fn()
const onOpen = vi.fn()

beforeEach(() => {
  onToggle.mockClear()
  onOpen.mockClear()
})

describe('<CategoryCard />', () => {
  it('renders name, safety badge, and item count + size', () => {
    render(
      <CategoryCard result={result()} itemCount={3} selected={false} maxSize={4096} onToggle={onToggle} onOpen={onOpen} />,
    )
    expect(screen.getByText('Trash')).toBeInTheDocument()
    expect(screen.getByText('Safe')).toBeInTheDocument()
    expect(screen.getByText('3 items · 2.0 KB')).toBeInTheDocument()
  })

  it('shows the scan error line when present', () => {
    render(
      <CategoryCard
        result={result({ error: 'permission denied' })}
        itemCount={0}
        selected={false}
        maxSize={4096}
        onToggle={onToggle}
        onOpen={onOpen}
      />,
    )
    expect(screen.getByText('permission denied')).toBeInTheDocument()
  })

  it('toggling the checkbox calls onToggle without opening the detail view', () => {
    render(
      <CategoryCard result={result()} itemCount={3} selected={false} maxSize={4096} onToggle={onToggle} onOpen={onOpen} />,
    )
    fireEvent.click(screen.getByLabelText('Select Trash'))
    expect(onToggle).toHaveBeenCalledTimes(1)
    expect(onOpen).not.toHaveBeenCalled()
  })

  it('clicking the row body opens the category detail', () => {
    render(
      <CategoryCard result={result()} itemCount={3} selected={false} maxSize={4096} onToggle={onToggle} onOpen={onOpen} />,
    )
    fireEvent.click(screen.getByText('Trash'))
    expect(onOpen).toHaveBeenCalledTimes(1)
    expect(onToggle).not.toHaveBeenCalled()
  })

  it('reflects the selected state on the checkbox', () => {
    render(
      <CategoryCard result={result()} itemCount={3} selected={true} maxSize={4096} onToggle={onToggle} onOpen={onOpen} />,
    )
    expect(screen.getByLabelText('Select Trash').getAttribute('aria-checked')).toBe('true')
  })
})
