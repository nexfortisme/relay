import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import {
  conversationStreamUrl,
  createConversation,
  createFailedMessage,
  createMessage,
  listMessages,
  requeueMessage,
  stopConversationGeneration,
} from '../lib/api'
import {
  parsePositiveInt,
  shouldAlertUploadFailure,
  validateSelectedFiles,
  type UploadLimits,
} from '../lib/uploadValidation'
import type { DisplayMessage } from '../types'
import { useConversationStore, DEFAULT_CONVERSATION_TITLE } from './conversationStore'

const maxSingleFileBytes = 50 * 1024 * 1024
const maxTotalUploadBytes = parsePositiveInt(
  import.meta.env.VITE_MAX_UPLOAD_BYTES,
  50 * 1024 * 1024,
)
const maxImageUploadBytes = parsePositiveInt(import.meta.env.VITE_MAX_IMAGE_BYTES, 15 * 1024 * 1024)
export const maxConversationTokenCount = parsePositiveInt(import.meta.env.VITE_MAX_TOKEN_COUNT, 0)
const uploadLimits: UploadLimits = {
  maxSingleFileBytes,
  maxTotalUploadBytes,
  maxImageUploadBytes,
}

const websocketConnecting = 0
const websocketOpen = 1

type StreamPayload = {
  type: string
  messageId?: string
  token?: string
  content?: string
  thinking?: string
  error?: string
  elapsedMs?: number
  inputTokens?: number
  outputTokens?: number
  reasoningTokens?: number
  totalTokens?: number
}

type QueuedStreamDelta = {
  conversationId: string
  messageId: string
  token: string
  thinking: string
}

function cloneMessages(items: DisplayMessage[]): DisplayMessage[] {
  return items.map((item) => ({ ...item }))
}

function pickLongestOrPrefix(cached: string, persisted: string): string {
  if (!cached) return persisted
  if (!persisted) return cached
  if (cached.startsWith(persisted) || persisted.startsWith(cached)) {
    return cached.length >= persisted.length ? cached : persisted
  }
  return persisted.length >= cached.length ? persisted : cached
}

function mergeMessagesPreservingStreamState(
  persisted: DisplayMessage[],
  cached: DisplayMessage[],
): DisplayMessage[] {
  if (cached.length === 0) return persisted

  const cachedById = new Map(cached.map((m) => [m.id, m]))
  const merged = persisted.map((message) => {
    const local = cachedById.get(message.id)
    if (!local || message.role !== 'assistant') return message
    return {
      ...message,
      content: pickLongestOrPrefix(local.content, message.content),
      thinking: pickLongestOrPrefix(local.thinking ?? '', message.thinking ?? '') || undefined,
      inputTokens: Math.max(local.inputTokens ?? 0, message.inputTokens ?? 0) || undefined,
      outputTokens: Math.max(local.outputTokens ?? 0, message.outputTokens ?? 0) || undefined,
      reasoningTokens:
        Math.max(local.reasoningTokens ?? 0, message.reasoningTokens ?? 0) || undefined,
      totalTokens: Math.max(local.totalTokens ?? 0, message.totalTokens ?? 0) || undefined,
    }
  })

  for (const localMessage of cached) {
    if (localMessage.id.startsWith('local-')) continue
    if (!merged.some((item) => item.id === localMessage.id)) {
      merged.push(localMessage)
    }
  }
  return merged
}

function isPersistedVersionOfLocalUserMessage(
  message: DisplayMessage,
  localMessage: DisplayMessage,
): boolean {
  return (
    message.role === 'user' &&
    !message.id.startsWith('local-') &&
    message.content === localMessage.content &&
    sameAttachmentNames(message.attachments, localMessage.attachments)
  )
}

function sameAttachmentNames(
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

function sumConversationTokens(items: DisplayMessage[]): number {
  return items.reduce((sum, message) => sum + positiveNumber(message.totalTokens), 0)
}

function positiveNumber(value: number | undefined): number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : 0
}

function applyTokenUsage(message: DisplayMessage, payload: StreamPayload) {
  if (typeof payload.inputTokens === 'number') message.inputTokens = payload.inputTokens
  if (typeof payload.outputTokens === 'number') message.outputTokens = payload.outputTokens
  if (typeof payload.reasoningTokens === 'number') message.reasoningTokens = payload.reasoningTokens
  if (typeof payload.totalTokens === 'number') message.totalTokens = payload.totalTokens
}

export const useChatStore = defineStore('chat', () => {
  const messages = ref<DisplayMessage[]>([])
  const conversationMessageCache = ref(new Map<string, DisplayMessage[]>())
  const draft = ref('')
  const selectedFiles = ref<File[]>([])
  const isSending = ref(false)
  const waitingForAssistantResponse = ref(false)
  const generatingConversationId = ref<string | null>(null)
  const waitingForAssistantConversationId = ref<string | null>(null)
  const streamError = ref('')

  let streamSocket: WebSocket | null = null
  let streamConversationId: string | null = null
  const pendingStreamDeltas = new Map<string, QueuedStreamDelta>()
  let streamFlushHandle: number | null = null

  const isSelectedConversationWaitingForAssistant = computed(() => {
    const convStore = useConversationStore()
    return (
      !!convStore.selectedConversationId &&
      waitingForAssistantResponse.value &&
      waitingForAssistantConversationId.value === convStore.selectedConversationId
    )
  })

  const shouldShowPendingAssistantPlaceholder = computed(
    () => isSelectedConversationWaitingForAssistant.value,
  )

  const conversationTokenCount = computed(() => sumConversationTokens(messages.value))

  const isConversationTokenCapReached = computed(
    () =>
      maxConversationTokenCount > 0 && conversationTokenCount.value >= maxConversationTokenCount,
  )

  async function initializeApp() {
    const convStore = useConversationStore()
    await convStore.loadConversations()
    const requestedConversationId = convStore.getConversationIdFromUrl()
    if (requestedConversationId) {
      const exists = convStore.conversations.find((c) => c.id === requestedConversationId)
      if (exists) {
        await selectConversation(requestedConversationId)
        return
      }
    }
    if (convStore.activeConversations.length === 0) {
      await handleCreateConversation()
      return
    }
    const first = convStore.activeConversations[0]
    if (!first) return
    await selectConversation(first.id)
  }

  async function handleCreateConversation() {
    const convStore = useConversationStore()
    const conversation = await createConversation()
    await convStore.loadConversations()
    convStore.pendingRouteConversationId = conversation.id
    await selectConversation(conversation.id, { updateUrl: false })
    convStore.updateConversationInUrl(null)
  }

  async function goHome() {
    const convStore = useConversationStore()
    const existingNewChat = convStore.activeConversations.find(
      (c) => c.title === DEFAULT_CONVERSATION_TITLE,
    )
    if (existingNewChat) {
      if (convStore.selectedConversationId !== existingNewChat.id) {
        await selectConversation(existingNewChat.id)
      }
      return
    }
    await handleCreateConversation()
  }

  async function selectConversation(
    conversationId: string,
    options?: { updateUrl?: boolean },
  ) {
    const convStore = useConversationStore()
    cacheCurrentConversationMessages()
    convStore.selectedConversationId = conversationId
    if (options?.updateUrl !== false) {
      convStore.updateConversationInUrl(conversationId)
    }
    messages.value = cloneMessages(conversationMessageCache.value.get(conversationId) ?? [])
    const persistedMessages = await listMessages(conversationId)
    messages.value = mergeMessagesPreservingStreamState(
      persistedMessages,
      conversationMessageCache.value.get(conversationId) ?? [],
    )
    conversationMessageCache.value.set(conversationId, cloneMessages(messages.value))
    convStore.renameDraft = convStore.selectedConversation?.title ?? ''
    convStore.isEditingTitle = false
    setupStream(conversationId)
  }

  function closeStream() {
    flushQueuedStreamDeltas()
    const socket = streamSocket
    streamSocket = null
    streamConversationId = null
    socket?.close()
  }

  function resumeSelectedConversationStream() {
    const convStore = useConversationStore()
    if (!convStore.selectedConversationId) return
    setupStream(convStore.selectedConversationId)
  }

  function setupStream(conversationId: string) {
    if (isActiveStreamFor(conversationId)) return
    closeStream()
    streamError.value = ''
    const socket = new WebSocket(conversationStreamUrl(conversationId))
    streamSocket = socket
    streamConversationId = conversationId

    socket.onmessage = (event) => {
      if (streamSocket !== socket) return
      handleStreamPayload(conversationId, JSON.parse(event.data) as StreamPayload)
    }

    socket.onerror = () => {
      if (streamSocket !== socket) return
      flushQueuedStreamDeltas()
      streamError.value = 'Stream disconnected'
      resetGenerationFor(conversationId)
      streamSocket = null
      streamConversationId = null
      socket.close()
    }

    socket.onclose = (event) => {
      if (streamSocket !== socket) return
      flushQueuedStreamDeltas()
      streamSocket = null
      streamConversationId = null
      if (event.wasClean) return
      streamError.value = `Stream closed (code ${event.code})`
      resetGenerationFor(conversationId)
    }
  }

  function isActiveStreamFor(conversationId: string): boolean {
    if (!streamSocket || streamConversationId !== conversationId) return false
    const readyState = (streamSocket as { readyState?: number }).readyState
    return readyState === undefined || readyState === websocketConnecting || readyState === websocketOpen
  }

  function handleStreamPayload(conversationId: string, payload: StreamPayload) {
    switch (payload.type) {
      case 'token':
      case 'thinking':
        queueStreamDelta(conversationId, payload)
        return
      case 'done':
      case 'stopped':
      case 'error':
        flushQueuedStreamDeltas()
        applyStreamPayload(conversationId, payload)
        return
      default:
        applyStreamPayload(conversationId, payload)
    }
  }

  function queueStreamDelta(conversationId: string, payload: StreamPayload) {
    if (!payload.messageId) return
    const token = payload.type === 'token' ? (payload.token ?? '') : ''
    const thinking = payload.type === 'thinking' ? (payload.thinking ?? '') : ''
    if (!token && !thinking) return

    clearAssistantWaitFor(conversationId)

    const key = `${conversationId}:${payload.messageId}`
    const existing = pendingStreamDeltas.get(key)
    if (existing) {
      existing.token += token
      existing.thinking += thinking
    } else {
      pendingStreamDeltas.set(key, { conversationId, messageId: payload.messageId, token, thinking })
    }
    scheduleStreamDeltaFlush()
  }

  function scheduleStreamDeltaFlush() {
    if (streamFlushHandle !== null) return
    if (typeof window.requestAnimationFrame === 'function') {
      streamFlushHandle = window.requestAnimationFrame(() => {
        streamFlushHandle = null
        flushQueuedStreamDeltas()
      })
      return
    }
    streamFlushHandle = window.setTimeout(() => {
      streamFlushHandle = null
      flushQueuedStreamDeltas()
    }, 16)
  }

  function flushQueuedStreamDeltas() {
    if (streamFlushHandle !== null) {
      if (typeof window.cancelAnimationFrame === 'function') {
        window.cancelAnimationFrame(streamFlushHandle as number)
      } else {
        window.clearTimeout(streamFlushHandle)
      }
      streamFlushHandle = null
    }
    if (pendingStreamDeltas.size === 0) return

    const queuedDeltas = [...pendingStreamDeltas.values()]
    pendingStreamDeltas.clear()
    for (const delta of queuedDeltas) {
      if (delta.token) upsertAssistantMessage(delta.conversationId, delta.messageId, delta.token)
      if (delta.thinking) upsertAssistantThinking(delta.conversationId, delta.messageId, delta.thinking)
    }
  }

  function startGenerationFor(conversationId: string) {
    isSending.value = true
    generatingConversationId.value = conversationId
    waitingForAssistantResponse.value = true
    waitingForAssistantConversationId.value = conversationId
    streamError.value = ''
  }

  function resetGenerationFor(conversationId: string) {
    if (generatingConversationId.value !== conversationId) return
    isSending.value = false
    generatingConversationId.value = null
    waitingForAssistantResponse.value = false
    waitingForAssistantConversationId.value = null
  }

  function applyStreamPayload(conversationId: string, payload: StreamPayload) {
    switch (payload.type) {
      case 'token':
        if (!payload.messageId) return
        clearAssistantWaitFor(conversationId)
        upsertAssistantMessage(conversationId, payload.messageId, payload.token ?? '')
        return
      case 'thinking':
        if (!payload.messageId) return
        clearAssistantWaitFor(conversationId)
        upsertAssistantThinking(conversationId, payload.messageId, payload.thinking ?? '')
        return
      case 'done':
      case 'stopped':
        finishAssistantStream(conversationId, payload)
        return
      case 'error':
        failAssistantStream(conversationId, payload.error ?? 'Stream error')
    }
  }

  function clearAssistantWaitFor(conversationId: string) {
    if (waitingForAssistantConversationId.value !== conversationId) return
    waitingForAssistantResponse.value = false
    waitingForAssistantConversationId.value = null
  }

  function finishAssistantStream(conversationId: string, payload: StreamPayload) {
    if (payload.messageId) finishAssistantMessage(conversationId, payload.messageId, payload)
    resetGenerationFor(conversationId)
  }

  function failAssistantStream(conversationId: string, message: string) {
    clearAssistantWaitFor(conversationId)
    streamError.value = message
    markLatestUserMessageError(conversationId)
    if (generatingConversationId.value === conversationId) {
      generatingConversationId.value = null
      isSending.value = false
    }
  }

  function upsertAssistantMessage(conversationId: string, messageId: string, token: string) {
    const targetMessages = ensureConversationMessages(conversationId)
    const existing = targetMessages.find((m) => m.id === messageId)
    if (existing) {
      existing.content += token
      syncVisibleMessagesFromConversation(conversationId)
      return
    }
    targetMessages.push({
      id: messageId,
      conversationId,
      role: 'assistant',
      content: token,
      createdAt: new Date().toISOString(),
    })
    syncVisibleMessagesFromConversation(conversationId)
  }

  function upsertAssistantThinking(conversationId: string, messageId: string, thinking: string) {
    if (!thinking) return
    const targetMessages = ensureConversationMessages(conversationId)
    const existing = targetMessages.find((m) => m.id === messageId)
    if (existing) {
      existing.thinking = (existing.thinking ?? '') + thinking
      syncVisibleMessagesFromConversation(conversationId)
      return
    }
    targetMessages.push({
      id: messageId,
      conversationId,
      role: 'assistant',
      content: '',
      thinking,
      createdAt: new Date().toISOString(),
    })
    syncVisibleMessagesFromConversation(conversationId)
  }

  function finishAssistantMessage(
    conversationId: string,
    messageId: string,
    payload: StreamPayload,
  ) {
    const targetMessages = ensureConversationMessages(conversationId)
    const existing = targetMessages.find((m) => m.id === messageId)
    if (!existing) {
      targetMessages.push({
        id: messageId,
        conversationId,
        role: 'assistant',
        content: payload.content ?? payload.token ?? '',
        thinking: payload.thinking || undefined,
        createdAt: new Date().toISOString(),
      })
      const created = targetMessages[targetMessages.length - 1]
      if (created) applyTerminalAssistantMetadata(created, payload)
      syncVisibleMessagesFromConversation(conversationId)
      return
    }
    if (typeof payload.content === 'string') {
      existing.content = pickLongestOrPrefix(existing.content, payload.content)
    }
    if (typeof payload.thinking === 'string') {
      existing.thinking = pickLongestOrPrefix(existing.thinking ?? '', payload.thinking) || undefined
    }
    applyTerminalAssistantMetadata(existing, payload)
    syncVisibleMessagesFromConversation(conversationId)
  }

  function applyTerminalAssistantMetadata(message: DisplayMessage, payload: StreamPayload) {
    if (typeof payload.elapsedMs === 'number') message.elapsedMs = payload.elapsedMs
    applyTokenUsage(message, payload)
  }

  async function sendMessage() {
    const convStore = useConversationStore()
    const content = draft.value.trim()
    if (!content || !convStore.selectedConversationId || isSending.value) return
    if (isConversationTokenCapReached.value) {
      streamError.value = `Conversation token cap reached (${conversationTokenCount.value.toLocaleString()}/${maxConversationTokenCount.toLocaleString()}). Start a new chat to continue.`
      return
    }

    const conversationId = convStore.selectedConversationId
    const filesToSend = [...selectedFiles.value]
    const localMessageId = appendOptimisticUserMessage(conversationId, content, filesToSend)
    draft.value = ''
    selectedFiles.value = []
    startGenerationFor(conversationId)

    try {
      await createMessage(conversationId, content, filesToSend)
      try {
        await reconcileSentUserMessage(conversationId, localMessageId)
      } catch (reconcileError) {
        console.error('failed to reconcile sent user message', reconcileError)
      }
      if (convStore.pendingRouteConversationId === conversationId) {
        convStore.updateConversationInUrl(conversationId)
        convStore.pendingRouteConversationId = null
      }
      await convStore.loadConversations()
    } catch (error) {
      await handleSendMessageFailure(conversationId, localMessageId, content, filesToSend, error)
    }
  }

  function appendOptimisticUserMessage(
    conversationId: string,
    content: string,
    files: File[],
  ): string {
    const localMessageId = `local-${Date.now()}`
    messages.value.push({
      id: localMessageId,
      conversationId,
      role: 'user',
      content,
      userContent: content,
      llmContent: content,
      attachments: files.map((file) => ({ id: '', name: file.name })),
      createdAt: new Date().toISOString(),
    })
    conversationMessageCache.value.set(conversationId, cloneMessages(messages.value))
    return localMessageId
  }

  async function handleSendMessageFailure(
    conversationId: string,
    localMessageId: string,
    content: string,
    files: File[],
    error: unknown,
  ) {
    resetGenerationFor(conversationId)
    const message = error instanceof Error ? error.message : 'Failed to send message'
    streamError.value = message
    await persistFailedLocalMessage(conversationId, localMessageId, content, files)
    if (shouldAlertUploadFailure(message, files)) {
      window.alert(message)
    }
  }

  async function persistFailedLocalMessage(
    conversationId: string,
    localMessageId: string,
    content: string,
    files: File[],
  ) {
    const localMessageIndex = messages.value.findIndex((item) => item.id === localMessageId)
    if (localMessageIndex < 0) return
    const localMessage = messages.value[localMessageIndex]
    if (!localMessage) return

    messages.value[localMessageIndex] = { ...localMessage, hasError: true }
    conversationMessageCache.value.set(conversationId, cloneMessages(messages.value))

    try {
      const persisted = await createFailedMessage(
        conversationId,
        content,
        files.map((file) => file.name),
      )
      messages.value[localMessageIndex] = persisted
      conversationMessageCache.value.set(conversationId, cloneMessages(messages.value))
      const convStore = useConversationStore()
      await convStore.loadConversations()
    } catch (persistError) {
      console.error('failed to persist failed user message', persistError)
    }
  }

  async function handleRequeueMessage(message: DisplayMessage) {
    const convStore = useConversationStore()
    const conversationId = convStore.selectedConversationId
    if (!conversationId || message.role !== 'user' || isSending.value) return

    startGenerationFor(conversationId)
    try {
      const { userMessage } = await requeueMessage(conversationId, message.id)
      messages.value.push(userMessage)
      conversationMessageCache.value.set(conversationId, cloneMessages(messages.value))
      const persistedMessages = await listMessages(conversationId)
      messages.value = mergeMessagesPreservingStreamState(
        persistedMessages,
        conversationMessageCache.value.get(conversationId) ?? [],
      )
      conversationMessageCache.value.set(conversationId, cloneMessages(messages.value))
      await convStore.loadConversations()
    } catch (error) {
      resetGenerationFor(conversationId)
      streamError.value = error instanceof Error ? error.message : 'Failed to requeue message'
    }
  }

  function handleSelectedFiles(files: File[]) {
    const error = validateSelectedFiles(files, uploadLimits)
    if (error) {
      selectedFiles.value = []
      notifyUploadError(error)
      return
    }
    selectedFiles.value = files
    clearResolvedUploadError()
  }

  function notifyUploadError(message: string) {
    streamError.value = message
    window.alert(message)
  }

  function clearResolvedUploadError() {
    if (
      streamError.value.includes('upload limit') ||
      streamError.value.includes('Image files must be')
    ) {
      streamError.value = ''
    }
  }

  function removeSelectedFile(index: number) {
    selectedFiles.value.splice(index, 1)
  }

  function setDraft(value: string) {
    draft.value = value
  }

  async function stopGeneration() {
    const convStore = useConversationStore()
    if (
      !convStore.selectedConversationId ||
      generatingConversationId.value !== convStore.selectedConversationId
    ) {
      return
    }
    await stopConversationGeneration(convStore.selectedConversationId)
  }

  async function archiveChat(conversationId: string, event?: MouseEvent) {
    if (event?.shiftKey) {
      await deleteChat(conversationId)
      return
    }
    const convStore = useConversationStore()
    if (!convStore.confirmArchive(conversationId)) return
    await convStore.archiveConversationById(conversationId)
    await moveSelectionAfterConversationLeavesList(conversationId)
  }

  async function archiveSelectedConversation(event?: MouseEvent) {
    const convStore = useConversationStore()
    if (!convStore.selectedConversationId) return
    if (event?.shiftKey) {
      await deleteChat(convStore.selectedConversationId)
      return
    }
    await archiveChat(convStore.selectedConversationId)
  }

  async function restoreChat(conversationId: string) {
    const convStore = useConversationStore()
    await convStore.restoreConversationById(conversationId)
  }

  async function deleteChat(conversationId: string) {
    const convStore = useConversationStore()
    if (!convStore.confirmDelete(conversationId)) return
    await convStore.deleteConversationById(conversationId)
    await moveSelectionAfterConversationLeavesList(conversationId)
  }

  async function moveSelectionAfterConversationLeavesList(conversationId: string) {
    const convStore = useConversationStore()
    if (convStore.selectedConversationId !== conversationId) return
    const replacement = convStore.activeConversations[0]
    if (replacement) {
      await selectConversation(replacement.id)
      return
    }
    messages.value = []
    convStore.selectedConversationId = null
    convStore.updateConversationInUrl(null)
  }

  function cacheCurrentConversationMessages() {
    const convStore = useConversationStore()
    if (!convStore.selectedConversationId) return
    conversationMessageCache.value.set(
      convStore.selectedConversationId,
      cloneMessages(messages.value),
    )
  }

  function markLatestUserMessageError(conversationId: string) {
    const latestUserMessage = [...messages.value]
      .reverse()
      .find((m) => m.conversationId === conversationId && m.role === 'user')
    if (!latestUserMessage) return
    latestUserMessage.hasError = true
  }

  function ensureConversationMessages(conversationId: string): DisplayMessage[] {
    const convStore = useConversationStore()
    const cached = conversationMessageCache.value.get(conversationId)
    if (cached) return cached
    const initial =
      conversationId === convStore.selectedConversationId ? cloneMessages(messages.value) : []
    conversationMessageCache.value.set(conversationId, initial)
    return initial
  }

  function syncVisibleMessagesFromConversation(conversationId: string) {
    const convStore = useConversationStore()
    if (conversationId !== convStore.selectedConversationId) return
    messages.value = cloneMessages(conversationMessageCache.value.get(conversationId) ?? [])
  }

  async function reconcileSentUserMessage(conversationId: string, localMessageId: string) {
    const cachedMessages = conversationMessageCache.value.get(conversationId) ?? []
    const localIndex = cachedMessages.findIndex((m) => m.id === localMessageId)
    if (localIndex < 0) return
    const localMessage = cachedMessages[localIndex]
    if (!localMessage) return
    const persistedMessages = await listMessages(conversationId)
    const persistedUserMessage = [...persistedMessages]
      .reverse()
      .find((m) => isPersistedVersionOfLocalUserMessage(m, localMessage))
    if (!persistedUserMessage) return

    const nextMessages = cloneMessages(cachedMessages)
    nextMessages[localIndex] = persistedUserMessage
    conversationMessageCache.value.set(conversationId, nextMessages)
    const convStore = useConversationStore()
    if (conversationId === convStore.selectedConversationId) {
      messages.value = cloneMessages(nextMessages)
    }
  }

  return {
    messages,
    conversationMessageCache,
    draft,
    selectedFiles,
    isSending,
    waitingForAssistantResponse,
    generatingConversationId,
    waitingForAssistantConversationId,
    streamError,
    isSelectedConversationWaitingForAssistant,
    shouldShowPendingAssistantPlaceholder,
    conversationTokenCount,
    isConversationTokenCapReached,
    maxConversationTokenCount,
    initializeApp,
    handleCreateConversation,
    goHome,
    selectConversation,
    closeStream,
    resumeSelectedConversationStream,
    sendMessage,
    handleRequeueMessage,
    handleSelectedFiles,
    removeSelectedFile,
    setDraft,
    stopGeneration,
    archiveChat,
    archiveSelectedConversation,
    restoreChat,
    deleteChat,
    cacheCurrentConversationMessages,
  }
})
