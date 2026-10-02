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

const moderate: Category = { ...category, id: 'system-cache', name: 'User Cache Files', safetyLevel: 'moderate' }

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
  it('renders name, item count and size; safe categories carry no badge', () => {
    render(<CategoryCard result={result()} itemCount={3} selected={false} onToggle={onToggle} onOpen={onOpen} />)
    expect(screen.getByText('Trash')).toBeInTheDocument()
    expect(screen.queryByText('Safe')).toBeNull()
    expect(screen.getByText('3 items')).toBeInTheDocument()
    expect(screen.getByText('2.0 KB')).toBeInTheDocument()
  })

  it('shows the safety badge for moderate and risky categories', () => {
    render(
      <CategoryCard result={result({ category: moderate })} itemCount={1} selected={false} onToggle={onToggle} onOpen={onOpen} />,
    )
    expect(screen.getByText('Moderate')).toBeInTheDocument()
    expect(screen.getByText('1 item')).toBeInTheDocument()
  })

  it('shows the scan error line when present', () => {
    render(
      <CategoryCard
        result={result({ error: 'permission denied' })}
        itemCount={0}
        selected={false}
        onToggle={onToggle}
        onOpen={onOpen}
      />,
    )
    expect(screen.getByText('permission denied')).toBeInTheDocument()
  })

  it('toggling the checkbox calls onToggle without opening the detail view', () => {
    render(<CategoryCard result={result()} itemCount={3} selected={false} onToggle={onToggle} onOpen={onOpen} />)
    fireEvent.click(screen.getByLabelText('Select Trash'))
    expect(onToggle).toHaveBeenCalledTimes(1)
    expect(onOpen).not.toHaveBeenCalled()
  })

  it('clicking the row body opens the category detail', () => {
    render(<CategoryCard result={result()} itemCount={3} selected={false} onToggle={onToggle} onOpen={onOpen} />)
    fireEvent.click(screen.getByText('Trash'))
    expect(onOpen).toHaveBeenCalledTimes(1)
    expect(onToggle).not.toHaveBeenCalled()
  })

  it('reflects the selected state on the checkbox', () => {
    render(<CategoryCard result={result()} itemCount={3} selected={true} onToggle={onToggle} onOpen={onOpen} />)
    expect(screen.getByLabelText('Select Trash').getAttribute('aria-checked')).toBe('true')
  })
})
