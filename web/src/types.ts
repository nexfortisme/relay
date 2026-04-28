import type { Message } from './lib/api'
import type { AttachmentPreviewKind } from './lib/fileTypes'

export type DisplayMessage = Message & {
  thinking?: string
  hasError?: boolean
}

export type FilePreviewState = {
  kind: AttachmentPreviewKind
  src: string
  downloadSrc: string
  filename: string
  text: string
  isLoading: boolean
  error: string
  objectUrl?: string
}
