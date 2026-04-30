import type { DisplayMessage } from '../types'

export function displayUserMessage(message: DisplayMessage): string {
  const fromUserContent = message.userContent?.trim()
  if (fromUserContent) {
    return fromUserContent
  }
  const divider = '\n\n---\n'
  const dividerIndex = message.content.indexOf(divider)
  if (dividerIndex >= 0) {
    return message.content.slice(0, dividerIndex).trim()
  }
  return message.content
}

export function formatElapsed(ms: number | undefined): string {
  if (typeof ms !== 'number' || !Number.isFinite(ms) || ms < 0) {
    return ''
  }
  if (ms < 1000) {
    return `${ms} ms`
  }
  const seconds = ms / 1000
  if (seconds < 60) {
    return `${seconds.toFixed(seconds < 10 ? 2 : 1)} s`
  }
  const totalSeconds = Math.round(seconds)
  const minutes = Math.floor(totalSeconds / 60)
  const remSeconds = totalSeconds % 60
  return `${minutes}m ${remSeconds}s`
}
