import { describe, expect, it } from 'vitest'
import { displayUserMessage, formatElapsed } from '../lib/messageFormatting'
import type { DisplayMessage } from '../types'

function userMessage(content: string, userContent?: string): DisplayMessage {
  return {
    id: 'msg-1',
    conversationId: 'conv-1',
    role: 'user',
    content,
    userContent,
    createdAt: '2026-04-25T12:00:00.000Z',
  }
}

describe('messageFormatting', () => {
  it('prefers stored user content and trims legacy attachment prompt dividers', () => {
    expect(displayUserMessage(userMessage('legacy prompt', ' visible prompt '))).toBe(
      'visible prompt',
    )
    expect(displayUserMessage(userMessage('visible\n\n---\ninternal attachment context'))).toBe(
      'visible',
    )
  })

  it('formats elapsed generation times for compact display', () => {
    expect(formatElapsed(undefined)).toBe('')
    expect(formatElapsed(-1)).toBe('')
    expect(formatElapsed(250)).toBe('250 ms')
    expect(formatElapsed(1500)).toBe('1.50 s')
    expect(formatElapsed(12_500)).toBe('12.5 s')
    expect(formatElapsed(65_000)).toBe('1m 5s')
  })
})
