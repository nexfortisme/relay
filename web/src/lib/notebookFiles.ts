import type { NotebookFile } from './notebooks'

export type ProcessingStage = { label: string; note?: string }

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

export function statusLabel(status: NotebookFile['status'] | string): string {
  switch (status) {
    case 'pending':
      return 'Queued'
    case 'processing':
      return 'Processing'
    case 'ready':
      return 'Ready'
    case 'error':
      return 'Error'
    default:
      return status
  }
}

export function formatNotebookDate(iso: string | undefined): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d.getTime())) return ''
  return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' }).format(d)
}

export function getProcessingStages(file: NotebookFile): ProcessingStage[] {
  if (file.fileKind === 'csv') {
    return [{ label: 'Parsing table structure' }, { label: 'Building search index' }]
  }
  if (file.fileKind === 'image') {
    return [{ label: 'Storing metadata' }]
  }

  const isPdf = file.contentType === 'application/pdf' || file.name.toLowerCase().endsWith('.pdf')
  if (isPdf) {
    return [
      { label: 'Extracting text' },
      { label: 'Rendering pages to images' },
      { label: 'Generating AI descriptions', note: 'may take a moment for large PDFs' },
      { label: 'Building search index' },
    ]
  }

  return [{ label: 'Extracting text' }, { label: 'Building search index' }]
}

export function formatElapsedSince(isoDate: string, nowMs = Date.now()): string {
  const secs = Math.max(0, Math.floor((nowMs - new Date(isoDate).getTime()) / 1000))
  if (secs < 60) return `${secs}s`
  const m = Math.floor(secs / 60)
  const s = secs % 60
  return `${m}m ${s}s`
}
