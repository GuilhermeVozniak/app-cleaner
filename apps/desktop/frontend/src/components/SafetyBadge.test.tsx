import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import SafetyBadge from './SafetyBadge'

describe('<SafetyBadge />', () => {
  it.each([
    ['safe', 'Safe'],
    ['moderate', 'Moderate'],
    ['risky', 'Risky'],
  ] as const)('renders the %s level with its label and tint', (level, label) => {
    render(<SafetyBadge level={level} />)
    const el = screen.getByText(label)
    expect(el.className).toContain(`text-${level}`)
  })
})
