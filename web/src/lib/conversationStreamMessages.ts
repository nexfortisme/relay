import type { DisplayMessage } from '../types'

/**
 * Shape of JSON events from GET /api/conversations/:id/stream (WebSocket).
 * The `type` field discriminates ping, deltas, and terminal payloads.
 */
export type ConversationStreamPayload = {
  type: string
  messageId?: string
  token?: string
  content?: string
  thinking?: string
  model?: string
  error?: string
  elapsedMs?: number
  inputTokens?: number
  outputTokens?: number
  reasoningTokens?: number
  totalTokens?: number
}

/**
 * Batched token/thinking deltas coalesced per animation frame so the UI
 * does not repaint on every single WebSocket message.
 */
export type QueuedAssistantStreamDelta = {
  conversationId: string
  messageId: string
  token: string
  thinking: string
  model?: string
}

/** Shallow copy of messages for optimistic cache snapshots. */
export function cloneDisplayMessages(items: DisplayMessage[]): DisplayMessage[] {
  return items.map((message) => ({ ...message }))
}

/**
 * When reconciling streamed assistant text with a later authoritative payload
 * (e.g. full `done` body), prefer the longer string when one extends the other
 * as a prefix; otherwise prefer the longer independent string so we never
 * regress partial stream content incorrectly.
 */
export function pickLongestOverlappingStreamText(
  streamedDraft: string,
  persistedCopy: string,
): string {
  if (!streamedDraft) return persistedCopy
  if (!persistedCopy) return streamedDraft
  if (
    streamedDraft.startsWith(persistedCopy) ||
    persistedCopy.startsWith(streamedDraft)
  ) {
    return streamedDraft.length >= persistedCopy.length ? streamedDraft : persistedCopy
  }
  return persistedCopy.length >= streamedDraft.length ? persistedCopy : streamedDraft
}

/**
 * After `listMessages`, merge persisted rows with the in-memory conversation
 * cache so in-flight streams are not wiped when refetching. Assistant rows
 * get content/thinking/token fields merged conservatively.
 */
export function mergePersistedWithCachedStreamMessages(
  persistedMessages: DisplayMessage[],
  cachedMessages: DisplayMessage[],
): DisplayMessage[] {
  if (cachedMessages.length === 0) {
    return persistedMessages
  }

  const cachedById = new Map(cachedMessages.map((message) => [message.id, message]))
  const merged = persistedMessages.map((persistedMessage) => {
    const cachedCopy = cachedById.get(persistedMessage.id)
    if (!cachedCopy || persistedMessage.role !== 'assistant') {
      return persistedMessage
    }
    return {
      ...persistedMessage,
      content: pickLongestOverlappingStreamText(cachedCopy.content, persistedMessage.content),
      thinking:
        pickLongestOverlappingStreamText(
          cachedCopy.thinking ?? '',
          persistedMessage.thinking ?? '',
        ) || undefined,
      model: persistedMessage.model || cachedCopy.model,
      inputTokens:
        Math.max(cachedCopy.inputTokens ?? 0, persistedMessage.inputTokens ?? 0) || undefined,
      outputTokens:
        Math.max(cachedCopy.outputTokens ?? 0, persistedMessage.outputTokens ?? 0) || undefined,
      reasoningTokens:
        Math.max(cachedCopy.reasoningTokens ?? 0, persistedMessage.reasoningTokens ?? 0) ||
        undefined,
      totalTokens:
        Math.max(cachedCopy.totalTokens ?? 0, persistedMessage.totalTokens ?? 0) || undefined,
    }
  })

  for (const cachedMessage of cachedMessages) {
    if (cachedMessage.id.startsWith('local-')) continue
    if (!merged.some((row) => row.id === cachedMessage.id)) {
      merged.push(cachedMessage)
    }
  }
  return merged
}

/**
 * Detects when a server-persisted user message is the counterpart of a
 * `local-{timestamp}` optimistic row so we can swap IDs without duplication.
 */
export function isPersistedMatchForOptimisticUserMessage(
  persistedMessage: DisplayMessage,
  optimisticMessage: DisplayMessage,
): boolean {
  return (
    persistedMessage.role === 'user' &&
    !persistedMessage.id.startsWith('local-') &&
    persistedMessage.content === optimisticMessage.content &&
    attachmentListsHaveSameDisplayNames(
      persistedMessage.attachments,
      optimisticMessage.attachments,
    )
  )
}

function attachmentListsHaveSameDisplayNames(
  left: { name: string }[] | undefined,
  right: { name: string }[] | undefined,
): boolean {
  const leftItems = left ?? []
  const rightItems = right ?? []
  return (
    leftItems.length === rightItems.length &&
    leftItems.every((item, index) => item.name === rightItems[index]?.name)
  )
}

/** Sums meaningful total token counts reported on messages for cap UI. */
export function sumTotalTokensAcrossMessages(items: DisplayMessage[]): number {
  return items.reduce((runningTotal, message) => {
    const n = message.totalTokens
    return runningTotal + (typeof n === 'number' && Number.isFinite(n) && n > 0 ? n : 0)
  }, 0)
}

export function applyTokenUsageFieldsFromPayload(
  message: DisplayMessage,
  payload: ConversationStreamPayload,
): void {
  if (typeof payload.inputTokens === 'number') message.inputTokens = payload.inputTokens
  if (typeof payload.outputTokens === 'number') message.outputTokens = payload.outputTokens
  if (typeof payload.reasoningTokens === 'number')
    message.reasoningTokens = payload.reasoningTokens
  if (typeof payload.totalTokens === 'number') message.totalTokens = payload.totalTokens
}
