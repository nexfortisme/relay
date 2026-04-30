export type UploadLimits = {
  maxSingleFileBytes: number
  maxTotalUploadBytes: number
  maxImageUploadBytes: number
}

export function parsePositiveInt(value: string | undefined, fallback: number): number {
  if (!value) {
    return fallback
  }
  const parsed = Number.parseInt(value, 10)
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallback
  }
  return parsed
}

export function formatBytesLabel(bytes: number): string {
  if (bytes >= 1024 * 1024) {
    const mb = bytes / (1024 * 1024)
    return Number.isInteger(mb) ? `${mb}MB` : `${mb.toFixed(1)}MB`
  }
  if (bytes >= 1024) {
    const kb = bytes / 1024
    return Number.isInteger(kb) ? `${kb}KB` : `${kb.toFixed(1)}KB`
  }
  return `${bytes}B`
}

export function shouldAlertUploadFailure(message: string, files: File[]): boolean {
  if (files.length === 0) {
    return false
  }
  const lower = message.toLowerCase()
  return (
    lower.includes('too large') ||
    lower.includes('upload limit') ||
    lower.includes('exceeds max size')
  )
}

export function validateSelectedFiles(files: File[], limits: UploadLimits): string | null {
  const oversizedFiles = files.filter((file) => file.size > limits.maxSingleFileBytes)
  if (oversizedFiles.length > 0) {
    return `Files must be ${formatBytesLabel(limits.maxSingleFileBytes)} or smaller: ${fileNames(oversizedFiles)}`
  }

  const totalBytes = files.reduce((sum, file) => sum + file.size, 0)
  if (totalBytes > limits.maxTotalUploadBytes) {
    return `Selected files exceed the ${formatBytesLabel(limits.maxTotalUploadBytes)} total upload limit. Remove some files and try again.`
  }

  const oversizedImages = files.filter(
    (file) => file.type.startsWith('image/') && file.size > limits.maxImageUploadBytes,
  )
  if (oversizedImages.length > 0) {
    return `Image files must be ${formatBytesLabel(limits.maxImageUploadBytes)} or smaller: ${fileNames(oversizedImages)}`
  }

  return null
}

function fileNames(files: File[]): string {
  return files.map((file) => file.name).join(', ')
}
