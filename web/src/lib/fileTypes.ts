export const ACCEPTED_FILE_TYPES = 'image/*,.pdf,.txt,.md,.markdown,.json,.csv,.xml,.yaml,.yml'

export type AttachmentPreviewKind = 'image' | 'pdf' | 'text' | 'markdown' | 'csv'

const IMAGE_EXTENSIONS = /\.(png|jpe?g|gif|webp|svg|bmp|ico|avif|tiff?)$/i
const MARKDOWN_EXTENSIONS = /\.(md|markdown)$/i
const PDF_EXTENSION = /\.pdf$/i
const CSV_EXTENSION = /\.csv$/i
const TEXT_EXTENSIONS = /\.(txt|json|xml|ya?ml)$/i
const JSON_EXTENSION = /\.json$/i

export function isImageFile(name: string): boolean {
  return IMAGE_EXTENSIONS.test(name)
}

export function attachmentPreviewKind(name: string): AttachmentPreviewKind | null {
  if (isImageFile(name)) {
    return 'image'
  }
  if (PDF_EXTENSION.test(name)) {
    return 'pdf'
  }
  if (MARKDOWN_EXTENSIONS.test(name)) {
    return 'markdown'
  }
  if (CSV_EXTENSION.test(name)) {
    return 'csv'
  }
  if (TEXT_EXTENSIONS.test(name)) {
    return 'text'
  }
  return null
}

export function isPreviewableAttachment(name: string): boolean {
  return attachmentPreviewKind(name) !== null
}

export function formatPreviewText(name: string, text: string): string {
  if (!JSON_EXTENSION.test(name)) {
    return text
  }

  try {
    return JSON.stringify(JSON.parse(text), null, 2)
  } catch {
    return text
  }
}
