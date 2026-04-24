import type { Message } from './lib/api'

export type DisplayMessage = Message & {
  thinking?: string
  hasError?: boolean
}
