import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import {
  archiveConversation,
  conversationStreamUrl,
  createConversation,
  createFailedMessage,
  createMessage,
  deleteConversation,
  getSettings,
  listConversations,
  listMessages,
  renameConversation,
  requeueMessage,
  restoreConversation,
  stopConversationGeneration,
  suggestConversationTitle,
  updateSettings,
  type Conversation,
  type Settings,
} from '../lib/api'
import type { DisplayMessage } from '../types'

export const DEFAULT_CONVERSATION_TITLE = 'New chat'

const MAX_CONVERSATION_TITLE_LENGTH = 40
const themeStorageKey = 'relay.theme'
const maxSingleFileBytes = 50 * 1024 * 1024
const maxTotalUploadBytes = parsePositiveInt(
  import.meta.env.VITE_MAX_UPLOAD_BYTES,
  50 * 1024 * 1024,
)
const maxImageUploadBytes = parsePositiveInt(import.meta.env.VITE_MAX_IMAGE_BYTES, 15 * 1024 * 1024)
const maxConversationTokenCount = parsePositiveInt(import.meta.env.VITE_MAX_TOKEN_COUNT, 0)
const maxSingleFileLabel = formatBytesLabel(maxSingleFileBytes)
const maxTotalUploadLabel = formatBytesLabel(maxTotalUploadBytes)
const maxImageUploadLabel = formatBytesLabel(maxImageUploadBytes)

type ConversationSelectionOptions = {
  updateUrl?: boolean
}

type StreamPayload = {
  type: string
  messageId?: string
  token?: string
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

function parsePositiveInt(value: string | undefined, fallback: number): number {
  if (!value) {
    return fallback
  }
  const parsed = Number.parseInt(value, 10)
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallback
  }
  return parsed
}

function formatBytesLabel(bytes: number): string {
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

function getStoredTheme(): 'dark' | 'light' {
  try {
    const stored = window.localStorage.getItem(themeStorageKey)
    return stored === 'light' ? 'light' : 'dark'
  } catch {
    return 'dark'
  }
}

function storeTheme(value: 'dark' | 'light') {
  try {
    window.localStorage.setItem(themeStorageKey, value)
  } catch {
    // Ignore storage write failures (private mode, blocked storage, etc).
  }
}

function cloneMessages(items: DisplayMessage[]): DisplayMessage[] {
  return items.map((item) => ({ ...item }))
}

function pickLongestOrPrefix(cached: string, persisted: string): string {
  if (!cached) {
    return persisted
  }
  if (!persisted) {
    return cached
  }
  if (cached.startsWith(persisted) || persisted.startsWith(cached)) {
    return cached.length >= persisted.length ? cached : persisted
  }
  return persisted.length >= cached.length ? persisted : cached
}

function mergeMessagesPreservingStreamState(
  persisted: DisplayMessage[],
  cached: DisplayMessage[],
): DisplayMessage[] {
  if (cached.length === 0) {
    return persisted
  }

  const cachedById = new Map(cached.map((message) => [message.id, message]))
  const merged = persisted.map((message) => {
    const local = cachedById.get(message.id)
    if (!local || message.role !== 'assistant') {
      return message
    }
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
    if (localMessage.id.startsWith('local-')) {
      continue
    }
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
    sameAttachments(message.attachments, localMessage.attachments)
  )
}

function sameAttachments(left: string[] | undefined, right: string[] | undefined): boolean {
  const leftItems = left ?? []
  const rightItems = right ?? []
  return (
    leftItems.length === rightItems.length &&
    leftItems.every((item, index) => item === rightItems[index])
  )
}

function sumConversationTokens(items: DisplayMessage[]): number {
  return items.reduce((sum, message) => sum + positiveNumber(message.totalTokens), 0)
}

function positiveNumber(value: number | undefined): number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : 0
}

function applyTokenUsage(message: DisplayMessage, payload: StreamPayload) {
  if (typeof payload.inputTokens === 'number') {
    message.inputTokens = payload.inputTokens
  }
  if (typeof payload.outputTokens === 'number') {
    message.outputTokens = payload.outputTokens
  }
  if (typeof payload.reasoningTokens === 'number') {
    message.reasoningTokens = payload.reasoningTokens
  }
  if (typeof payload.totalTokens === 'number') {
    message.totalTokens = payload.totalTokens
  }
}

function clampTitleForDisplay(title: string): string {
  const normalized = title.trim().replace(/\s+/g, ' ')
  if (normalized.length <= MAX_CONVERSATION_TITLE_LENGTH) {
    return normalized
  }
  return normalized.slice(0, MAX_CONVERSATION_TITLE_LENGTH).trim()
}

function shouldAlertUploadFailure(message: string, files: File[]): boolean {
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

function validateSelectedFiles(files: File[]): string | null {
  const oversizedFiles = files.filter((file) => file.size > maxSingleFileBytes)
  if (oversizedFiles.length > 0) {
    return `Files must be ${maxSingleFileLabel} or smaller: ${oversizedFiles.map((file) => file.name).join(', ')}`
  }
  const totalBytes = files.reduce((sum, file) => sum + file.size, 0)
  if (totalBytes > maxTotalUploadBytes) {
    return `Selected files exceed the ${maxTotalUploadLabel} total upload limit. Remove some files and try again.`
  }
  const oversizedImages = files.filter(
    (file) => file.type.startsWith('image/') && file.size > maxImageUploadBytes,
  )
  if (oversizedImages.length > 0) {
    return `Image files must be ${maxImageUploadLabel} or smaller: ${oversizedImages.map((file) => file.name).join(', ')}`
  }
  return null
}

export const useAppStore = defineStore('app', () => {
  const conversations = ref<Conversation[]>([])
  const selectedConversationId = ref<string | null>(null)
  const pendingRouteConversationId = ref<string | null>(null)
  const messages = ref<DisplayMessage[]>([])
  const conversationMessageCache = ref(new Map<string, DisplayMessage[]>())
  const draft = ref('')
  const selectedFiles = ref<File[]>([])
  const isSending = ref(false)
  const waitingForAssistantResponse = ref(false)
  const generatingConversationId = ref<string | null>(null)
  const waitingForAssistantConversationId = ref<string | null>(null)
  const streamError = ref('')
  const showArchived = ref(false)
  const isSidebarCollapsed = ref(false)
  const theme = ref<'dark' | 'light'>(getStoredTheme())
  const showSettings = ref(false)
  const settingsForm = ref<Settings>({ llm_url: '', llm_model: '', system_prompt: '' })
  const settingsSaving = ref(false)
  const settingsError = ref('')
  const renameDraft = ref('')
  const isRenaming = ref(false)
  const isSuggestingTitle = ref(false)
  const isEditingTitle = ref(false)
  let streamSocket: WebSocket | null = null
  const pendingStreamDeltas = new Map<string, QueuedStreamDelta>()
  let streamFlushHandle: number | null = null

  const selectedConversation = computed(() =>
    conversations.value.find((conversation) => conversation.id === selectedConversationId.value),
  )
  const activeConversations = computed(() =>
    conversations.value.filter((conversation) => !conversation.archived),
  )
  const isSelectedConversationWaitingForAssistant = computed(
    () =>
      !!selectedConversationId.value &&
      waitingForAssistantResponse.value &&
      waitingForAssistantConversationId.value === selectedConversationId.value,
  )
  const shouldShowPendingAssistantPlaceholder = computed(
    () => isSelectedConversationWaitingForAssistant.value,
  )
  const conversationTokenCount = computed(() => sumConversationTokens(messages.value))
  const isConversationTokenCapReached = computed(
    () =>
      maxConversationTokenCount > 0 && conversationTokenCount.value >= maxConversationTokenCount,
  )

  async function initializeApp() {
    await loadConversations()
    const requestedConversationId = getConversationIdFromUrl()
    if (requestedConversationId) {
      const requestedConversation = conversations.value.find(
        (conversation) => conversation.id === requestedConversationId,
      )
      if (requestedConversation) {
        await selectConversation(requestedConversation.id)
        return
      }
    }
    if (activeConversations.value.length === 0) {
      await handleCreateConversation()
      return
    }
    const firstConversation = activeConversations.value[0]
    if (!firstConversation) {
      return
    }
    await selectConversation(firstConversation.id)
  }

  function closeStream() {
    flushQueuedStreamDeltas()
    streamSocket?.close()
    streamSocket = null
  }

  async function loadConversations() {
    conversations.value = await listConversations(true)
  }

  async function handleCreateConversation() {
    const conversation = await createConversation()
    await loadConversations()
    pendingRouteConversationId.value = conversation.id
    await selectConversation(conversation.id, { updateUrl: false })
    updateConversationInUrl(null)
  }

  async function goHome() {
    const existingNewChat = activeConversations.value.find(
      (conversation) => conversation.title === DEFAULT_CONVERSATION_TITLE,
    )
    if (existingNewChat) {
      if (selectedConversationId.value !== existingNewChat.id) {
        await selectConversation(existingNewChat.id)
      }
      return
    }
    await handleCreateConversation()
  }

  async function selectConversation(
    conversationId: string,
    options?: ConversationSelectionOptions,
  ) {
    cacheCurrentConversationMessages()
    selectedConversationId.value = conversationId
    if (options?.updateUrl !== false) {
      updateConversationInUrl(conversationId)
    }
    messages.value = cloneMessages(conversationMessageCache.value.get(conversationId) ?? [])
    const persistedMessages = await listMessages(conversationId)
    messages.value = mergeMessagesPreservingStreamState(
      persistedMessages,
      conversationMessageCache.value.get(conversationId) ?? [],
    )
    conversationMessageCache.value.set(conversationId, cloneMessages(messages.value))
    renameDraft.value = selectedConversation.value?.title ?? ''
    isEditingTitle.value = false
    setupStream(conversationId)
  }

  function getConversationIdFromUrl(): string | null {
    const url = new URL(window.location.href)
    return url.searchParams.get('conversation')
  }

  function updateConversationInUrl(conversationId: string | null) {
    const url = new URL(window.location.href)
    if (conversationId) {
      url.searchParams.set('conversation', conversationId)
    } else {
      url.searchParams.delete('conversation')
    }
    const nextUrl = `${url.pathname}${url.search}${url.hash}`
    window.history.replaceState(window.history.state, '', nextUrl)
  }

  function setupStream(conversationId: string) {
    closeStream()
    streamError.value = ''
    streamSocket = new WebSocket(conversationStreamUrl(conversationId))

    streamSocket.onmessage = (event) => {
      handleStreamPayload(conversationId, JSON.parse(event.data) as StreamPayload)
    }

    streamSocket.onerror = () => {
      flushQueuedStreamDeltas()
      streamError.value = 'Stream disconnected'
      resetGenerationFor(conversationId)
    }

    streamSocket.onclose = (event) => {
      flushQueuedStreamDeltas()
      if (event.wasClean) {
        return
      }
      streamError.value = `Stream closed (code ${event.code})`
      resetGenerationFor(conversationId)
    }
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
    if (!payload.messageId) {
      return
    }
    const token = payload.type === 'token' ? (payload.token ?? '') : ''
    const thinking = payload.type === 'thinking' ? (payload.thinking ?? '') : ''
    if (!token && !thinking) {
      return
    }

    clearAssistantWaitFor(conversationId)

    const key = `${conversationId}:${payload.messageId}`
    const existing = pendingStreamDeltas.get(key)
    if (existing) {
      existing.token += token
      existing.thinking += thinking
    } else {
      pendingStreamDeltas.set(key, {
        conversationId,
        messageId: payload.messageId,
        token,
        thinking,
      })
    }
    scheduleStreamDeltaFlush()
  }

  function scheduleStreamDeltaFlush() {
    if (streamFlushHandle !== null) {
      return
    }
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
    if (pendingStreamDeltas.size === 0) {
      return
    }

    const queuedDeltas = [...pendingStreamDeltas.values()]
    pendingStreamDeltas.clear()
    for (const delta of queuedDeltas) {
      if (delta.token) {
        upsertAssistantMessage(delta.conversationId, delta.messageId, delta.token)
      }
      if (delta.thinking) {
        upsertAssistantThinking(delta.conversationId, delta.messageId, delta.thinking)
      }
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
    if (generatingConversationId.value !== conversationId) {
      return
    }
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
    if (waitingForAssistantConversationId.value !== conversationId) {
      return
    }
    waitingForAssistantResponse.value = false
    waitingForAssistantConversationId.value = null
  }

  function finishAssistantStream(conversationId: string, payload: StreamPayload) {
    if (payload.messageId) {
      finishAssistantMessage(conversationId, payload.messageId, payload)
    }
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
    const existing = targetMessages.find((message) => message.id === messageId)
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
    if (!thinking) {
      return
    }
    const targetMessages = ensureConversationMessages(conversationId)
    const existing = targetMessages.find((message) => message.id === messageId)
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
    const existing = targetMessages.find((message) => message.id === messageId)
    if (!existing) {
      return
    }
    if (typeof payload.elapsedMs === 'number') {
      existing.elapsedMs = payload.elapsedMs
    }
    applyTokenUsage(existing, payload)
    syncVisibleMessagesFromConversation(conversationId)
  }

  function beginConversationTitleEdit() {
    renameDraft.value = selectedConversation.value?.title ?? DEFAULT_CONVERSATION_TITLE
    isEditingTitle.value = true
  }

  function cancelConversationTitleEdit() {
    isEditingTitle.value = false
    renameDraft.value = selectedConversation.value?.title ?? DEFAULT_CONVERSATION_TITLE
  }

  async function sendMessage() {
    const content = draft.value.trim()
    if (!content || !selectedConversationId.value || isSending.value) {
      return
    }
    if (isConversationTokenCapReached.value) {
      streamError.value = `Conversation token cap reached (${conversationTokenCount.value.toLocaleString()}/${maxConversationTokenCount.toLocaleString()}). Start a new chat to continue.`
      return
    }

    const conversationId = selectedConversationId.value
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
      if (pendingRouteConversationId.value === conversationId) {
        updateConversationInUrl(conversationId)
        pendingRouteConversationId.value = null
      }
      await loadConversations()
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
      attachments: files.map((file) => file.name),
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
    if (localMessageIndex < 0) {
      return
    }
    const localMessage = messages.value[localMessageIndex]
    if (!localMessage) {
      return
    }

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
      await loadConversations()
    } catch (persistError) {
      console.error('failed to persist failed user message', persistError)
    }
  }

  async function handleRequeueMessage(message: DisplayMessage) {
    const conversationId = selectedConversationId.value
    if (!conversationId || message.role !== 'user' || isSending.value) {
      return
    }

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
      await loadConversations()
    } catch (error) {
      resetGenerationFor(conversationId)
      streamError.value = error instanceof Error ? error.message : 'Failed to requeue message'
    }
  }

  function handleSelectedFiles(files: File[]) {
    const error = validateSelectedFiles(files)
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

  function toggleTheme() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
    storeTheme(theme.value)
  }

  function toggleArchived() {
    showArchived.value = !showArchived.value
  }

  function toggleSidebarCollapsed() {
    isSidebarCollapsed.value = !isSidebarCollapsed.value
  }

  async function openSettings() {
    settingsError.value = ''
    try {
      const settings = await getSettings()
      settingsForm.value = { ...settings }
    } catch {
      settingsForm.value = { llm_url: '', llm_model: '', system_prompt: '' }
    }
    showSettings.value = true
  }

  function closeSettings() {
    showSettings.value = false
  }

  async function saveSettings() {
    settingsSaving.value = true
    settingsError.value = ''
    try {
      const saved = await updateSettings(settingsForm.value)
      settingsForm.value = { ...saved }
      showSettings.value = false
    } catch (error) {
      settingsError.value = error instanceof Error ? error.message : 'Failed to save settings'
    } finally {
      settingsSaving.value = false
    }
  }

  async function saveConversationTitle() {
    if (!selectedConversationId.value) {
      return
    }
    const title = clampTitleForDisplay(renameDraft.value)
    if (!title) {
      renameDraft.value = selectedConversation.value?.title ?? DEFAULT_CONVERSATION_TITLE
      isEditingTitle.value = false
      return
    }
    isRenaming.value = true
    try {
      renameDraft.value = title
      await renameConversation(selectedConversationId.value, title)
      await loadConversations()
      isEditingTitle.value = false
    } finally {
      isRenaming.value = false
    }
  }

  async function suggestConversationTitleWithLLM() {
    if (!selectedConversationId.value || isRenaming.value || isSuggestingTitle.value) {
      return
    }
    isSuggestingTitle.value = true
    try {
      const suggestedTitle = await suggestConversationTitle(selectedConversationId.value)
      renameDraft.value = clampTitleForDisplay(suggestedTitle)
      await saveConversationTitle()
    } catch (error) {
      streamError.value =
        error instanceof Error ? error.message : 'Failed to suggest conversation title'
    } finally {
      isSuggestingTitle.value = false
    }
  }

  async function stopGeneration() {
    if (
      !selectedConversationId.value ||
      generatingConversationId.value !== selectedConversationId.value
    ) {
      return
    }
    await stopConversationGeneration(selectedConversationId.value)
  }

  function cacheCurrentConversationMessages() {
    if (!selectedConversationId.value) {
      return
    }
    conversationMessageCache.value.set(selectedConversationId.value, cloneMessages(messages.value))
  }

  function markLatestUserMessageError(conversationId: string) {
    const latestUserMessage = [...messages.value]
      .reverse()
      .find((message) => message.conversationId === conversationId && message.role === 'user')
    if (!latestUserMessage) {
      return
    }
    latestUserMessage.hasError = true
  }

  function ensureConversationMessages(conversationId: string): DisplayMessage[] {
    const cached = conversationMessageCache.value.get(conversationId)
    if (cached) {
      return cached
    }
    const initial =
      conversationId === selectedConversationId.value ? cloneMessages(messages.value) : []
    conversationMessageCache.value.set(conversationId, initial)
    return initial
  }

  function syncVisibleMessagesFromConversation(conversationId: string) {
    if (conversationId !== selectedConversationId.value) {
      return
    }
    messages.value = cloneMessages(conversationMessageCache.value.get(conversationId) ?? [])
  }

  async function reconcileSentUserMessage(conversationId: string, localMessageId: string) {
    const cachedMessages = conversationMessageCache.value.get(conversationId) ?? []
    const localIndex = cachedMessages.findIndex((message) => message.id === localMessageId)
    if (localIndex < 0) {
      return
    }
    const localMessage = cachedMessages[localIndex]
    if (!localMessage) {
      return
    }
    const persistedMessages = await listMessages(conversationId)
    const persistedUserMessage = [...persistedMessages]
      .reverse()
      .find((message) => isPersistedVersionOfLocalUserMessage(message, localMessage))
    if (!persistedUserMessage) {
      return
    }

    const nextMessages = cloneMessages(cachedMessages)
    nextMessages[localIndex] = persistedUserMessage
    conversationMessageCache.value.set(conversationId, nextMessages)
    if (conversationId === selectedConversationId.value) {
      messages.value = cloneMessages(nextMessages)
    }
  }

  function confirmArchive(conversationId: string): boolean {
    const conversation = conversations.value.find((item) => item.id === conversationId)
    const title = conversation?.title ?? 'this chat'
    return window.confirm(`Archive "${title}"?`)
  }

  function confirmDelete(conversationId: string): boolean {
    const conversation = conversations.value.find((item) => item.id === conversationId)
    const title = conversation?.title ?? 'this chat'
    return window.confirm(`Delete "${title}"? This cannot be undone.`)
  }

  async function archiveSelectedConversation(event?: MouseEvent) {
    if (!selectedConversationId.value) {
      return
    }
    if (event?.shiftKey) {
      await deleteChat(selectedConversationId.value)
      return
    }
    await archiveChat(selectedConversationId.value)
  }

  async function archiveChat(conversationId: string, event?: MouseEvent) {
    if (event?.shiftKey) {
      await deleteChat(conversationId)
      return
    }
    if (!confirmArchive(conversationId)) {
      return
    }
    await archiveConversation(conversationId)
    await loadConversations()
    await moveSelectionAfterConversationLeavesList(conversationId)
  }

  async function restoreChat(conversationId: string) {
    await restoreConversation(conversationId)
    await loadConversations()
  }

  async function deleteChat(conversationId: string) {
    if (!confirmDelete(conversationId)) {
      return
    }
    await deleteConversation(conversationId)
    await loadConversations()
    await moveSelectionAfterConversationLeavesList(conversationId)
  }

  async function moveSelectionAfterConversationLeavesList(conversationId: string) {
    if (selectedConversationId.value !== conversationId) {
      return
    }
    const replacement = activeConversations.value[0]
    if (replacement) {
      await selectConversation(replacement.id)
      return
    }
    messages.value = []
    selectedConversationId.value = null
    updateConversationInUrl(null)
  }

  return {
    conversations,
    selectedConversationId,
    pendingRouteConversationId,
    messages,
    conversationMessageCache,
    draft,
    selectedFiles,
    isSending,
    waitingForAssistantResponse,
    generatingConversationId,
    waitingForAssistantConversationId,
    streamError,
    showArchived,
    isSidebarCollapsed,
    theme,
    showSettings,
    settingsForm,
    settingsSaving,
    settingsError,
    renameDraft,
    isRenaming,
    isSuggestingTitle,
    isEditingTitle,
    selectedConversation,
    activeConversations,
    isSelectedConversationWaitingForAssistant,
    shouldShowPendingAssistantPlaceholder,
    conversationTokenCount,
    isConversationTokenCapReached,
    maxConversationTokenCount,
    initializeApp,
    closeStream,
    loadConversations,
    handleCreateConversation,
    goHome,
    selectConversation,
    beginConversationTitleEdit,
    cancelConversationTitleEdit,
    sendMessage,
    handleRequeueMessage,
    handleSelectedFiles,
    removeSelectedFile,
    setDraft,
    toggleTheme,
    toggleArchived,
    toggleSidebarCollapsed,
    openSettings,
    closeSettings,
    saveSettings,
    saveConversationTitle,
    suggestConversationTitleWithLLM,
    stopGeneration,
    archiveSelectedConversation,
    archiveChat,
    restoreChat,
    deleteChat,
  }
})
