<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import ChatComposer from './components/ChatComposer.vue'
import ChatHeader from './components/ChatHeader.vue'
import ConversationSidebar from './components/ConversationSidebar.vue'
import MessageList from './components/MessageList.vue'
import SettingsModal from './components/SettingsModal.vue'
import {
  archiveConversation,
  conversationHttpStreamUrl,
  conversationStreamUrl,
  createConversation,
  createFailedMessage,
  deleteConversation,
  createMessage,
  getSettings,
  updateSettings,
  listConversations,
  listMessages,
  renameConversation,
  requeueMessage,
  suggestConversationTitle,
  restoreConversation,
  stopConversationGeneration,
  type Conversation,
  type Settings,
} from './lib/api'
import type { DisplayMessage } from './types'

const conversations = ref<Conversation[]>([])
const selectedConversationId = ref<string | null>(null)
const pendingRouteConversationId = ref<string | null>(null)

const messages = ref<DisplayMessage[]>([])
const conversationMessageCache = new Map<string, DisplayMessage[]>()
const draft = ref('')
const selectedFiles = ref<File[]>([])
const isSending = ref(false)
const waitingForAssistantResponse = ref(false)
const generatingConversationId = ref<string | null>(null)
const waitingForAssistantConversationId = ref<string | null>(null)
const streamError = ref('')
const showArchived = ref(false)
const themeStorageKey = 'relay.theme'
const theme = ref<'dark' | 'light'>(getStoredTheme())
const showSettings = ref(false)
const settingsForm = ref<Settings>({ llm_url: '', llm_model: '', system_prompt: '' })
const settingsSaving = ref(false)
const settingsError = ref('')
const renameDraft = ref('')
const isRenaming = ref(false)
const isSuggestingTitle = ref(false)
const isEditingTitle = ref(false)
const messageListEl = ref<InstanceType<typeof MessageList> | null>(null)
let streamSocket: WebSocket | null = null
let eventSourceFallback: EventSource | null = null
const maxTotalUploadBytes = parsePositiveInt(import.meta.env.VITE_MAX_UPLOAD_BYTES, 50 * 1024 * 1024)
const maxTotalUploadLabel = formatBytesLabel(maxTotalUploadBytes)
const maxSingleFileBytes = 50 * 1024 * 1024
const maxSingleFileLabel = formatBytesLabel(maxSingleFileBytes)
const maxImageUploadBytes = parsePositiveInt(import.meta.env.VITE_MAX_IMAGE_BYTES, 15 * 1024 * 1024)
const maxImageUploadLabel = formatBytesLabel(maxImageUploadBytes)

const selectedConversation = computed(() =>
  conversations.value.find((c) => c.id === selectedConversationId.value),
)
const activeConversations = computed(() => conversations.value.filter((c) => !c.archived))
const isSelectedConversationGenerating = computed(
  () => !!selectedConversationId.value && generatingConversationId.value === selectedConversationId.value,
)
const isSelectedConversationWaitingForAssistant = computed(
  () =>
    !!selectedConversationId.value &&
    waitingForAssistantResponse.value &&
    waitingForAssistantConversationId.value === selectedConversationId.value,
)
const hasSelectedConversationAssistantOutput = computed(() =>
  messages.value.some((message) => {
    if (message.role !== 'assistant') {
      return false
    }
    if (message.content.trim().length > 0) {
      return true
    }
    return (message.thinking ?? '').trim().length > 0
  }),
)
const shouldShowPendingAssistantPlaceholder = computed(
  () => isSelectedConversationWaitingForAssistant.value,
)

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

function notifyUploadError(message: string) {
  streamError.value = message
  window.alert(message)
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

onMounted(async () => {
  await loadConversations()
  const requestedConversationId = getConversationIdFromUrl()
  if (requestedConversationId) {
    const requestedConversation = conversations.value.find((conversation) => conversation.id === requestedConversationId)
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
})

onUnmounted(() => {
  if (streamSocket) {
    streamSocket.close()
  }
  if (eventSourceFallback) {
    eventSourceFallback.close()
  }
})

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

async function selectConversation(conversationId: string, options?: { updateUrl?: boolean }) {
  cacheCurrentConversationMessages()
  selectedConversationId.value = conversationId
  if (options?.updateUrl !== false) {
    updateConversationInUrl(conversationId)
  }
  messages.value = cloneMessages(conversationMessageCache.get(conversationId) ?? [])
  const persistedMessages = await listMessages(conversationId)
  messages.value = mergeMessagesPreservingStreamState(
    persistedMessages,
    conversationMessageCache.get(conversationId) ?? [],
  )
  conversationMessageCache.set(conversationId, cloneMessages(messages.value))
  renameDraft.value = selectedConversation.value?.title ?? ''
  isEditingTitle.value = false
  setupStream(conversationId)
  await scrollMessagesToBottom()
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

type StreamPayload = {
  type: string
  messageId?: string
  token?: string
  thinking?: string
  error?: string
}

function setupStream(conversationId: string) {
  if (streamSocket) {
    streamSocket.close()
  }
  if (eventSourceFallback) {
    eventSourceFallback.close()
    eventSourceFallback = null
  }
  streamError.value = ''
  let hasOpenedWebSocket = false
  streamSocket = new WebSocket(conversationStreamUrl(conversationId))

  streamSocket.onopen = () => {
    hasOpenedWebSocket = true
  }

  streamSocket.onmessage = (event) => {
    const payload = JSON.parse(event.data) as StreamPayload
    applyStreamPayload(conversationId, payload)
  }

  streamSocket.onerror = () => {
    if (!hasOpenedWebSocket) {
      setupEventSourceFallback(conversationId)
      return
    }
    streamError.value = 'Stream disconnected'
    if (generatingConversationId.value === conversationId) {
      isSending.value = false
      generatingConversationId.value = null
      waitingForAssistantResponse.value = false
      waitingForAssistantConversationId.value = null
    }
  }

  streamSocket.onclose = (event) => {
    if (!hasOpenedWebSocket) {
      setupEventSourceFallback(conversationId)
      return
    }
    if (!event.wasClean) {
      streamError.value = `Stream closed (code ${event.code})`
      if (generatingConversationId.value === conversationId) {
        isSending.value = false
        generatingConversationId.value = null
        waitingForAssistantResponse.value = false
        waitingForAssistantConversationId.value = null
      }
    }
  }
}

function applyStreamPayload(conversationId: string, payload: StreamPayload) {
  if (payload.type === 'token' && payload.messageId) {
    if (waitingForAssistantConversationId.value === conversationId) {
      waitingForAssistantResponse.value = false
      waitingForAssistantConversationId.value = null
    }
    upsertAssistantMessage(conversationId, payload.messageId, payload.token ?? '')
    void scrollMessagesToBottom()
    return
  }

  if (payload.type === 'thinking' && payload.messageId) {
    if (waitingForAssistantConversationId.value === conversationId) {
      waitingForAssistantResponse.value = false
      waitingForAssistantConversationId.value = null
    }
    upsertAssistantThinking(conversationId, payload.messageId, payload.thinking ?? '')
    return
  }

  if (payload.type === 'done') {
    if (generatingConversationId.value === conversationId) {
      waitingForAssistantResponse.value = false
      waitingForAssistantConversationId.value = null
      generatingConversationId.value = null
      isSending.value = false
    }
    return
  }

  if (payload.type === 'stopped') {
    if (generatingConversationId.value === conversationId) {
      waitingForAssistantResponse.value = false
      waitingForAssistantConversationId.value = null
      generatingConversationId.value = null
      isSending.value = false
    }
    return
  }

  if (payload.type === 'error') {
    if (waitingForAssistantConversationId.value === conversationId) {
      waitingForAssistantResponse.value = false
      waitingForAssistantConversationId.value = null
    }
    streamError.value = payload.error ?? 'Stream error'
    markLatestUserMessageError(conversationId)
    if (generatingConversationId.value === conversationId) {
      generatingConversationId.value = null
      isSending.value = false
    }
  }
}

function setupEventSourceFallback(conversationId: string) {
  if (eventSourceFallback) {
    eventSourceFallback.close()
  }
  eventSourceFallback = new EventSource(conversationHttpStreamUrl(conversationId))
  streamError.value = 'WebSocket failed, using fallback stream'

  eventSourceFallback.addEventListener('token', (event) => {
    const payload = JSON.parse((event as MessageEvent).data) as StreamPayload
    applyStreamPayload(conversationId, payload)
  })
  eventSourceFallback.addEventListener('thinking', (event) => {
    const payload = JSON.parse((event as MessageEvent).data) as StreamPayload
    applyStreamPayload(conversationId, payload)
  })
  eventSourceFallback.addEventListener('done', (event) => {
    const payload = JSON.parse((event as MessageEvent).data) as StreamPayload
    applyStreamPayload(conversationId, payload)
  })
  eventSourceFallback.addEventListener('error', (event) => {
    const messageEvent = event as MessageEvent
    const payload = safeParseStreamPayload(messageEvent.data)
    applyStreamPayload(conversationId, payload)
    streamError.value = payload.error ?? 'Stream disconnected'
    if (generatingConversationId.value === conversationId) {
      isSending.value = false
      generatingConversationId.value = null
      waitingForAssistantResponse.value = false
      waitingForAssistantConversationId.value = null
    }
  })
}

function safeParseStreamPayload(raw: unknown): StreamPayload {
  if (typeof raw !== 'string') {
    return { type: 'error', error: 'Stream disconnected' }
  }
  try {
    return JSON.parse(raw) as StreamPayload
  } catch {
    return { type: 'error', error: 'Stream disconnected' }
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

async function beginConversationTitleEdit() {
  renameDraft.value = selectedConversation.value?.title ?? 'New chat'
  isEditingTitle.value = true
}

function cancelConversationTitleEdit() {
  isEditingTitle.value = false
  renameDraft.value = selectedConversation.value?.title ?? 'New chat'
}

async function sendMessage() {
  const content = draft.value.trim()
  if (!content || !selectedConversationId.value || isSending.value) {
    return
  }

  const conversationId = selectedConversationId.value
  const filesToSend = [...selectedFiles.value]
  const localMessageId = `local-${Date.now()}`
  messages.value.push({
    id: localMessageId,
    conversationId,
    role: 'user',
    content,
    userContent: content,
    llmContent: content,
    attachments: filesToSend.map((file) => file.name),
    createdAt: new Date().toISOString(),
  })
  conversationMessageCache.set(conversationId, cloneMessages(messages.value))

  draft.value = ''
  selectedFiles.value = []
  isSending.value = true
  generatingConversationId.value = conversationId
  waitingForAssistantResponse.value = true
  waitingForAssistantConversationId.value = conversationId
  try {
    await createMessage(conversationId, content, filesToSend)
    if (pendingRouteConversationId.value === conversationId) {
      updateConversationInUrl(conversationId)
      pendingRouteConversationId.value = null
    }
    await loadConversations()
    await scrollMessagesToBottom()
  } catch (error) {
    if (generatingConversationId.value === conversationId) {
      isSending.value = false
      generatingConversationId.value = null
    }
    if (waitingForAssistantConversationId.value === conversationId) {
      waitingForAssistantResponse.value = false
      waitingForAssistantConversationId.value = null
    }
    const message = error instanceof Error ? error.message : 'Failed to send message'
    streamError.value = message
    const localMessageIndex = messages.value.findIndex((item) => item.id === localMessageId)
    if (localMessageIndex >= 0) {
      const localMessage = messages.value[localMessageIndex]
      if (localMessage) {
        messages.value[localMessageIndex] = {
          ...localMessage,
          hasError: true,
        }
        conversationMessageCache.set(conversationId, cloneMessages(messages.value))
        try {
          const persisted = await createFailedMessage(
            conversationId,
            content,
            filesToSend.map((file) => file.name),
          )
          messages.value[localMessageIndex] = persisted
          conversationMessageCache.set(conversationId, cloneMessages(messages.value))
          await loadConversations()
        } catch (persistError) {
          console.error('failed to persist failed user message', persistError)
        }
      }
    }
    const lower = message.toLowerCase()
    if (
      filesToSend.length > 0 &&
      (lower.includes('too large') || lower.includes('upload limit') || lower.includes('exceeds max size'))
    ) {
      window.alert(message)
    }
  }
}

async function handleRequeueMessage(message: DisplayMessage) {
  const conversationId = selectedConversationId.value
  if (!conversationId || message.role !== 'user' || isSending.value) {
    return
  }

  isSending.value = true
  generatingConversationId.value = conversationId
  waitingForAssistantResponse.value = true
  waitingForAssistantConversationId.value = conversationId
  streamError.value = ''
  try {
    const { userMessage } = await requeueMessage(conversationId, message.id)
    messages.value.push(userMessage)
    conversationMessageCache.set(conversationId, cloneMessages(messages.value))
    const persistedMessages = await listMessages(conversationId)
    messages.value = mergeMessagesPreservingStreamState(persistedMessages, conversationMessageCache.get(conversationId) ?? [])
    conversationMessageCache.set(conversationId, cloneMessages(messages.value))
    await loadConversations()
    await scrollMessagesToBottom()
  } catch (error) {
    if (generatingConversationId.value === conversationId) {
      isSending.value = false
      generatingConversationId.value = null
    }
    if (waitingForAssistantConversationId.value === conversationId) {
      waitingForAssistantResponse.value = false
      waitingForAssistantConversationId.value = null
    }
    streamError.value = error instanceof Error ? error.message : 'Failed to requeue message'
  }
}

function handleSelectedFiles(files: File[]) {
  const oversizedFiles = files.filter((file) => file.size > maxSingleFileBytes)
  if (oversizedFiles.length > 0) {
    selectedFiles.value = []
    notifyUploadError(
      `Files must be ${maxSingleFileLabel} or smaller: ${oversizedFiles.map((file) => file.name).join(', ')}`,
    )
    return
  }
  const totalBytes = files.reduce((sum, file) => sum + file.size, 0)
  if (totalBytes > maxTotalUploadBytes) {
    selectedFiles.value = []
    notifyUploadError(
      `Selected files exceed the ${maxTotalUploadLabel} total upload limit. Remove some files and try again.`,
    )
    return
  }
  const oversizedImages = files.filter(
    (file) => file.type.startsWith('image/') && file.size > maxImageUploadBytes,
  )
  if (oversizedImages.length > 0) {
    selectedFiles.value = []
    notifyUploadError(
      `Image files must be ${maxImageUploadLabel} or smaller: ${oversizedImages.map((file) => file.name).join(', ')}`,
    )
    return
  }
  selectedFiles.value = files
  if (streamError.value.includes('upload limit') || streamError.value.includes('Image files must be')) {
    streamError.value = ''
  }
}

function removeSelectedFile(index: number) {
  selectedFiles.value.splice(index, 1)
}

function toggleTheme() {
  theme.value = theme.value === 'dark' ? 'light' : 'dark'
  storeTheme(theme.value)
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

async function openSettings() {
  settingsError.value = ''
  try {
    const s = await getSettings()
    settingsForm.value = { ...s }
  } catch {
    settingsForm.value = { llm_url: '', llm_model: '', system_prompt: '' }
  }
  showSettings.value = true
}

async function saveSettings() {
  settingsSaving.value = true
  settingsError.value = ''
  try {
    const saved = await updateSettings(settingsForm.value)
    settingsForm.value = { ...saved }
    showSettings.value = false
  } catch (e) {
    settingsError.value = e instanceof Error ? e.message : 'Failed to save settings'
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
    renameDraft.value = selectedConversation.value?.title ?? 'New chat'
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

function clampTitleForDisplay(title: string): string {
  const normalized = title.trim().replace(/\s+/g, ' ')
  if (normalized.length <= 40) {
    return normalized
  }
  return normalized.slice(0, 40).trim()
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
    streamError.value = error instanceof Error ? error.message : 'Failed to suggest conversation title'
  } finally {
    isSuggestingTitle.value = false
  }
}

async function stopGeneration() {
  if (!selectedConversationId.value || generatingConversationId.value !== selectedConversationId.value) {
    return
  }
  await stopConversationGeneration(selectedConversationId.value)
}

function cacheCurrentConversationMessages() {
  if (!selectedConversationId.value) {
    return
  }
  conversationMessageCache.set(selectedConversationId.value, cloneMessages(messages.value))
}

function ensureConversationMessages(conversationId: string): DisplayMessage[] {
  const cached = conversationMessageCache.get(conversationId)
  if (cached) {
    return cached
  }
  const initial = conversationId === selectedConversationId.value ? cloneMessages(messages.value) : []
  conversationMessageCache.set(conversationId, initial)
  return initial
}

function syncVisibleMessagesFromConversation(conversationId: string) {
  if (conversationId !== selectedConversationId.value) {
    return
  }
  messages.value = cloneMessages(conversationMessageCache.get(conversationId) ?? [])
}

function cloneMessages(items: DisplayMessage[]): DisplayMessage[] {
  return items.map((item) => ({ ...item }))
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
  const toArchive = selectedConversationId.value
  if (!confirmArchive(toArchive)) {
    return
  }
  await archiveConversation(toArchive)
  await loadConversations()
  if (selectedConversationId.value === toArchive) {
    const replacement = activeConversations.value[0]
    if (replacement) {
      await selectConversation(replacement.id)
    } else {
      messages.value = []
      selectedConversationId.value = null
      updateConversationInUrl(null)
    }
  }
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
  if (selectedConversationId.value === conversationId) {
    const replacement = activeConversations.value[0]
    if (replacement) {
      await selectConversation(replacement.id)
    } else {
      messages.value = []
      selectedConversationId.value = null
      updateConversationInUrl(null)
    }
  }
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
  if (selectedConversationId.value === conversationId) {
    const replacement = activeConversations.value[0]
    if (replacement) {
      await selectConversation(replacement.id)
    } else {
      messages.value = []
      selectedConversationId.value = null
      updateConversationInUrl(null)
    }
  }
}

async function scrollMessagesToBottom() {
  const list = messageListEl.value
  if (list && 'scrollToBottom' in list && typeof list.scrollToBottom === 'function') {
    await list.scrollToBottom()
  }
}
</script>

<template>
  <main class="layout" :data-theme="theme">
    <ConversationSidebar
      :conversations="conversations"
      :generating-conversation-id="generatingConversationId"
      :selected-conversation-id="selectedConversationId"
      :show-archived="showArchived"
      :theme="theme"
      @archive="archiveChat"
      @create="handleCreateConversation"
      @delete="deleteChat"
      @open-settings="openSettings"
      @restore="restoreChat"
      @select="selectConversation"
      @toggle-archived="showArchived = !showArchived"
      @toggle-theme="toggleTheme"
    />

    <section class="chat-panel">
      <ChatHeader
        v-model:rename-draft="renameDraft"
        :is-editing="isEditingTitle"
        :is-renaming="isRenaming"
        :is-suggesting-title="isSuggestingTitle"
        :selected-conversation-id="selectedConversationId"
        :title="selectedConversation?.title ?? 'New chat'"
        @archive="archiveSelectedConversation"
        @begin-edit="beginConversationTitleEdit"
        @cancel-edit="cancelConversationTitleEdit"
        @save-title="saveConversationTitle"
        @suggest-title="suggestConversationTitleWithLLM"
      />
      <MessageList
        ref="messageListEl"
        :messages="messages"
        :pending-assistant="shouldShowPendingAssistantPlaceholder"
        :requeue-disabled="isSending"
        @requeue="handleRequeueMessage"
      />
      <p v-if="streamError" class="error">{{ streamError }}</p>
      <ChatComposer
        :draft="draft"
        :is-sending="isSending"
        :selected-files="selectedFiles"
        @remove-file="removeSelectedFile"
        @send="sendMessage"
        @stop="stopGeneration"
        @update-draft="draft = $event"
        @update-files="handleSelectedFiles"
      />
    </section>
    <SettingsModal
      v-if="showSettings"
      v-model:settings="settingsForm"
      :error="settingsError"
      :saving="settingsSaving"
      @close="showSettings = false"
      @save="saveSettings"
    />
  </main>
</template>

<style scoped>
:global(html, body, #app) {
  margin: 0;
  height: 100%;
  overflow: hidden;
}

.layout {
  display: grid;
  grid-template-columns: 300px 1fr;
  height: 100dvh;
  overflow: hidden;
  background: var(--bg);
  color: var(--text);
  font-family:
    Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}

.layout[data-theme='dark'] {
  --bg: #0f1115;
  --sidebar: #151821;
  --surface: #191d27;
  --surface-soft: #202533;
  --surface-hover: #262c3a;
  --selected: #222b3f;
  --text: #f4f7fb;
  --muted: #9aa5b5;
  --border: #2b3240;
  --primary: #3b82f6;
  --primary-strong: #2563eb;
  --danger: #dc2626;
  --danger-strong: #b91c1c;
  --shadow: 0 18px 46px rgba(0, 0, 0, 0.34);
}

.layout[data-theme='light'] {
  --bg: #f6f7f9;
  --sidebar: #ffffff;
  --surface: #ffffff;
  --surface-soft: #f1f3f6;
  --surface-hover: #e9edf2;
  --selected: #eef4ff;
  --text: #111827;
  --muted: #667085;
  --border: #d9dee7;
  --primary: #2563eb;
  --primary-strong: #1d4ed8;
  --danger: #dc2626;
  --danger-strong: #b91c1c;
  --shadow: 0 18px 46px rgba(31, 41, 55, 0.16);
}

.chat-panel {
  display: grid;
  grid-template-rows: auto 1fr auto;
  overflow: hidden;
  background: var(--bg);
  position: relative;
}

.error {
  color: #ef4444;
  padding: 0 1.25rem 0.5rem;
}
</style>
