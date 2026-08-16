import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import EmptyState from './EmptyState'

describe('<EmptyState />', () => {
  it('renders title and subtitle', () => {
    render(<EmptyState title="Nothing here" subtitle="All clean." />)
    expect(screen.getByText('Nothing here')).toBeInTheDocument()
    expect(screen.getByText('All clean.')).toBeInTheDocument()
  })

  it('omits the subtitle when not given', () => {
    render(<EmptyState title="Nothing here" />)
    expect(screen.getByText('Nothing here')).toBeInTheDocument()
    expect(screen.queryByText('All clean.')).toBeNull()
  })
})
