import { describe, expect, it } from 'vitest'
import { renderMarkdown } from '../lib/markdown'

describe('renderMarkdown', () => {
  it('wraps tables for chat grid styling', () => {
    const host = document.createElement('div')
    host.innerHTML = renderMarkdown('| Name | Address |\n| --- | --- |\n| Ada | Alexandria |')

    expect(host.querySelector('.markdown-table-wrap > table')).not.toBeNull()
  })
})
