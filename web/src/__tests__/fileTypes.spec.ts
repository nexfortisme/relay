import { describe, expect, it } from 'vitest'
import { ACCEPTED_FILE_TYPES, attachmentPreviewKind, formatPreviewText } from '../lib/fileTypes'

describe('fileTypes', () => {
  it('classifies every accepted non-wildcard extension for preview', () => {
    expect(ACCEPTED_FILE_TYPES).toBe(
      'image/*,.pdf,.txt,.md,.markdown,.json,.csv,.xml,.yaml,.yml',
    )
    expect(attachmentPreviewKind('photo.png')).toBe('image')
    expect(attachmentPreviewKind('paper.pdf')).toBe('pdf')
    expect(attachmentPreviewKind('notes.txt')).toBe('text')
    expect(attachmentPreviewKind('readme.md')).toBe('markdown')
    expect(attachmentPreviewKind('readme.markdown')).toBe('markdown')
    expect(attachmentPreviewKind('data.json')).toBe('text')
    expect(attachmentPreviewKind('rows.csv')).toBe('text')
    expect(attachmentPreviewKind('payload.xml')).toBe('text')
    expect(attachmentPreviewKind('config.yaml')).toBe('text')
    expect(attachmentPreviewKind('config.yml')).toBe('text')
  })

  it('formats JSON previews when possible', () => {
    expect(formatPreviewText('data.json', '{"name":"relay","count":2}')).toBe(
      '{\n  "name": "relay",\n  "count": 2\n}',
    )
    expect(formatPreviewText('data.json', '{bad')).toBe('{bad')
  })
})
