import { describe, expect, it } from 'vitest'
import type { NotebookFile } from '../lib/notebooks'
import {
  formatBytes,
  formatElapsedSince,
  getProcessingStages,
  statusLabel,
} from '../lib/notebookFiles'

function file(overrides: Partial<NotebookFile>): NotebookFile {
  return {
    id: 'file-1',
    notebookId: 'notebook-1',
    name: 'notes.txt',
    contentType: 'text/plain',
    sizeBytes: 42,
    fileKind: 'document',
    status: 'ready',
    pageCount: 0,
    pagesIndexed: 0,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('notebookFiles', () => {
  it('formats file sizes and statuses', () => {
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(1536)).toBe('1.5 KB')
    expect(formatBytes(2 * 1024 * 1024)).toBe('2.0 MB')
    expect(statusLabel('pending')).toBe('Queued')
    expect(statusLabel('ready')).toBe('Ready')
  })

  it('describes processing stages by file kind', () => {
    expect(getProcessingStages(file({ fileKind: 'csv', name: 'rows.csv' })).map((s) => s.label)).toEqual([
      'Parsing table structure',
      'Building search index',
    ])
    expect(
      getProcessingStages(
        file({ contentType: 'application/pdf', name: 'scan.pdf' }),
      ).map((s) => s.label),
    ).toContain('Generating AI descriptions')
  })

  it('formats elapsed processing time from a stable clock', () => {
    expect(formatElapsedSince('2026-01-01T00:00:00Z', Date.parse('2026-01-01T00:00:05Z'))).toBe(
      '5s',
    )
    expect(formatElapsedSince('2026-01-01T00:00:00Z', Date.parse('2026-01-01T00:01:05Z'))).toBe(
      '1m 5s',
    )
  })
})
