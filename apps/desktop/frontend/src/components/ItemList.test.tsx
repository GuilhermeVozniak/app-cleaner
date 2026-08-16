import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'

vi.mock('../../wailsjs/go/main/App', () => ({
  RevealInFinder: vi.fn(),
  CopyPath: vi.fn(),
  GetHome: vi.fn().mockResolvedValue(null),
}))

import { RevealInFinder, CopyPath } from '../../wailsjs/go/main/App'
import ItemList from './ItemList'
import type { CleanableItem } from '../lib/types'

const RevealMock = RevealInFinder as unknown as ReturnType<typeof vi.fn>
const CopyMock = CopyPath as unknown as ReturnType<typeof vi.fn>

const items: CleanableItem[] = [
  { path: '/tmp/a.log', size: 1024, name: 'a.log', isDirectory: false },
  { path: '/tmp/b.log', size: 2048, name: 'b.log', isDirectory: false },
]

const onToggle = vi.fn()

beforeEach(() => {
  onToggle.mockClear()
  RevealMock.mockClear()
  CopyMock.mockClear()
})

describe('<ItemList />', () => {
  it('renders name, path, and formatted size per row', () => {
    render(<ItemList items={items} selectable={true} isChecked={() => false} onToggle={onToggle} />)
    expect(screen.getByText('a.log')).toBeInTheDocument()
    expect(screen.getByText('1.0 KB')).toBeInTheDocument()
    expect(screen.getByText('2.0 KB')).toBeInTheDocument()
  })

  it('checkbox toggles report the row path', () => {
    render(<ItemList items={items} selectable={true} isChecked={(p) => p === '/tmp/a.log'} onToggle={onToggle} />)
    expect(screen.getByLabelText('Select a.log').getAttribute('aria-checked')).toBe('true')
    expect(screen.getByLabelText('Select b.log').getAttribute('aria-checked')).toBe('false')
    fireEvent.click(screen.getByLabelText('Select b.log'))
    expect(onToggle).toHaveBeenCalledWith('/tmp/b.log')
  })

  it('hides checkboxes when not selectable (docker rows)', () => {
    render(<ItemList items={items} selectable={false} isChecked={() => false} onToggle={onToggle} />)
    expect(screen.queryByLabelText('Select a.log')).toBeNull()
  })

  it('Reveal in Finder / Copy path call the bindings with the row path', () => {
    render(<ItemList items={items} selectable={true} isChecked={() => false} onToggle={onToggle} />)
    fireEvent.click(screen.getAllByTitle('Reveal in Finder')[0])
    expect(RevealMock).toHaveBeenCalledWith('/tmp/a.log')
    fireEvent.click(screen.getAllByTitle('Copy path')[1])
    expect(CopyMock).toHaveBeenCalledWith('/tmp/b.log')
  })
})
