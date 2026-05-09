import { describe, expect, it } from 'vitest'
import {
  mergePersistedWithCachedStreamMessages,
  pickLongestOverlappingStreamText,
} from '../lib/conversationStreamMessages'
import type { DisplayMessage } from '../types'

function assistantRow(
  overrides: Partial<DisplayMessage> & Pick<DisplayMessage, 'id' | 'conversationId' | 'createdAt'>,
): DisplayMessage {
  return {
    role: 'assistant',
    content: '',
    thinking: undefined,
    model: undefined,
    ...overrides,
  }
}

describe('pickLongestOverlappingStreamText', () => {
  it('returns the streamed draft when persisted is shorter prefix', () => {
    expect(pickLongestOverlappingStreamText('hello world', 'hello')).toBe('hello world')
  })

  it('returns persisted when streamed is shorter prefix', () => {
    expect(pickLongestOverlappingStreamText('hello', 'hello world')).toBe('hello world')
  })

  it('falls back to longer string when neither is a prefix', () => {
    expect(pickLongestOverlappingStreamText('ab', 'cd')).toBe('cd')
    expect(pickLongestOverlappingStreamText('abcd', 'cd')).toBe('abcd')
  })
})

describe('mergePersistedWithCachedStreamMessages', () => {
  it('fills assistant content from cache when persisted is stale mid-stream', () => {
    const persisted = [
      assistantRow({
        id: 'asst-1',
        conversationId: 'c1',
        content: '',
        createdAt: 't0',
      }),
    ]
    const cached = [
      assistantRow({
        id: 'asst-1',
        conversationId: 'c1',
        content: 'partial transcript',
        createdAt: 't0',
      }),
    ]
    const merged = mergePersistedWithCachedStreamMessages(persisted, cached)
    expect(merged[0]?.content).toBe('partial transcript')
  })

  it('preserves orphan cached rows that are absent from persisted list', () => {
    const persisted: DisplayMessage[] = []
    const cached = [
      assistantRow({
        id: 'asst-extra',
        conversationId: 'c1',
        content: 'streaming only',
        createdAt: 't0',
      }),
    ]
    const merged = mergePersistedWithCachedStreamMessages(persisted, cached)
    expect(merged.map((message) => message.id)).toEqual(['asst-extra'])
  })

  it('drops messages whose ids begin with local- from orphan merge', () => {
    const cached: DisplayMessage[] = [
      {
        id: 'local-user',
        conversationId: 'c1',
        role: 'user',
        content: 'x',
        createdAt: 't0',
      },
    ]
    expect(mergePersistedWithCachedStreamMessages([], cached)).toEqual([])
  })
})
