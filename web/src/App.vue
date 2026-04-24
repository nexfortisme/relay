<script setup lang="ts">
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
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
  messageAttachmentDownloadUrl,
  renameConversation,
  restoreConversation,
  stopConversationGeneration,
  type Conversation,
  type Message,
  type Settings,
} from './lib/api'

const conversations = ref<Conversation[]>([])
const selectedConversationId = ref<string | null>(null)
type DisplayMessage = Message & { thinking?: string; hasError?: boolean }

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
const theme = ref<'dark' | 'light'>('dark')
const showSettings = ref(false)
const settingsForm = ref<Settings>({ llm_url: '', llm_model: '', system_prompt: '' })
const settingsSaving = ref(false)
const settingsError = ref('')
const renameDraft = ref('')
const isRenaming = ref(false)
const isEditingTitle = ref(false)
const titleInputEl = ref<HTMLInputElement | null>(null)
const fileInputEl = ref<HTMLInputElement | null>(null)
const messagesEl = ref<HTMLElement | null>(null)
let streamSocket: WebSocket | null = null
let eventSourceFallback: EventSource | null = null
const maxTotalUploadBytes = parsePositiveInt(import.meta.env.VITE_MAX_UPLOAD_BYTES, 30 * 1024 * 1024)
const maxTotalUploadLabel = formatBytesLabel(maxTotalUploadBytes)
const maxImageUploadBytes = parsePositiveInt(import.meta.env.VITE_MAX_IMAGE_BYTES, 15 * 1024 * 1024)
const maxImageUploadLabel = formatBytesLabel(maxImageUploadBytes)

const selectedConversation = computed(() =>
  conversations.value.find((c) => c.id === selectedConversationId.value),
)
const activeConversations = computed(() => conversations.value.filter((c) => !c.archived))
const archivedConversations = computed(() => conversations.value.filter((c) => c.archived))
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
  () => isSelectedConversationGenerating.value && !hasSelectedConversationAssistantOutput.value,
)

const displayMessages = computed(() => messages.value)

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

function displayUserMessage(message: DisplayMessage): string {
  const fromUserContent = message.userContent?.trim()
  if (fromUserContent) {
    return fromUserContent
  }
  const legacy = message.content
  const divider = '\n\n---\n'
  const dividerIndex = legacy.indexOf(divider)
  if (dividerIndex >= 0) {
    return legacy.slice(0, dividerIndex).trim()
  }
  return legacy
}

marked.setOptions({
  gfm: true,
  breaks: true,
})

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
  await selectConversation(conversation.id)
}

async function selectConversation(conversationId: string) {
  cacheCurrentConversationMessages()
  selectedConversationId.value = conversationId
  updateConversationInUrl(conversationId)
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
  await nextTick()
  titleInputEl.value?.focus()
  titleInputEl.value?.select()
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
  if (fileInputEl.value) {
    fileInputEl.value.value = ''
  }
  isSending.value = true
  generatingConversationId.value = conversationId
  waitingForAssistantResponse.value = true
  waitingForAssistantConversationId.value = conversationId
  try {
    await createMessage(conversationId, content, filesToSend)
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

function handleFileSelection(event: Event) {
  const input = event.target as HTMLInputElement
  const files = input.files ? Array.from(input.files) : []
  const totalBytes = files.reduce((sum, file) => sum + file.size, 0)
  if (totalBytes > maxTotalUploadBytes) {
    selectedFiles.value = []
    input.value = ''
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
    input.value = ''
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

function openFilePicker() {
  fileInputEl.value?.click()
}

function toggleTheme() {
  theme.value = theme.value === 'dark' ? 'light' : 'dark'
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
  const title = renameDraft.value.trim()
  if (!title) {
    renameDraft.value = selectedConversation.value?.title ?? 'New chat'
    isEditingTitle.value = false
    return
  }
  isRenaming.value = true
  try {
    await renameConversation(selectedConversationId.value, title)
    await loadConversations()
    isEditingTitle.value = false
  } finally {
    isRenaming.value = false
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
  await nextTick()
  const el = messagesEl.value
  if (!el) {
    return
  }
  el.scrollTop = el.scrollHeight
}

function renderAssistantMarkdown(content: string): string {
  const parsed = marked.parse(content, { async: false })
  return DOMPurify.sanitize(parsed)
}

function renderThinkingMarkdown(content: string): string {
  const parsed = marked.parse(content, { async: false })
  return DOMPurify.sanitize(parsed)
}

function attachmentDownloadUrl(message: Message, attachmentIndex: number): string {
  return messageAttachmentDownloadUrl(message.conversationId, message.id, attachmentIndex)
}
</script>

<template>
  <main class="layout" :data-theme="theme">
    <aside class="sidebar">
      <div class="sidebar-actions">
        <button class="new-chat" @click="handleCreateConversation">New Chat</button>
        <div class="sidebar-controls">
          <button
            class="control-btn"
            :title="showArchived ? 'Hide archived' : 'Show archived'"
            @click="showArchived = !showArchived"
          >
            {{ showArchived ? '📂' : '📁' }}
          </button>
          <button
            class="control-btn"
            :title="theme === 'dark' ? 'Light mode' : 'Dark mode'"
            @click="toggleTheme"
          >
            {{ theme === 'dark' ? '☀️' : '🌙' }}
          </button>
          <button class="control-btn" title="Settings" @click="openSettings">⚙️</button>
        </div>
      </div>
      <div class="conversation-list">
        <div
          v-for="conversation in activeConversations"
          :key="conversation.id"
          class="conversation-row"
          :class="{ active: conversation.id === selectedConversationId }"
        >
          <button class="conversation-item" @click="selectConversation(conversation.id)">
            {{ conversation.title }}
          </button>
          <span
            v-if="generatingConversationId === conversation.id"
            class="sidebar-generating-indicator"
            aria-label="Generating response"
            title="Generating response"
          />
          <button
            class="icon-button"
            title="Archive chat (Shift+click to delete)"
            @click.stop="archiveChat(conversation.id, $event)"
          >
            📦
          </button>
        </div>
        <div v-if="showArchived" class="archived-section">
          <div class="archived-title">Archived chats</div>
          <div
            v-for="conversation in archivedConversations"
            :key="conversation.id"
            class="conversation-row archived"
          >
            <button class="conversation-item" @click="selectConversation(conversation.id)">
              {{ conversation.title }}
            </button>
            <button class="icon-button" title="Restore chat" @click.stop="restoreChat(conversation.id)">
              ↩
            </button>
            <button class="icon-button" title="Delete chat" @click.stop="deleteChat(conversation.id)">
              🗑
            </button>
          </div>
        </div>
      </div>
    </aside>

    <section class="chat-panel">
      <header class="chat-header">
        <div class="chat-title-wrap">
          <template v-if="!isEditingTitle">
            <h1 class="chat-title">{{ selectedConversation?.title ?? 'New chat' }}</h1>
            <button class="title-edit-button" :disabled="isRenaming" @click="beginConversationTitleEdit">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path
                  d="M3 17.25V21h3.75L17.8 9.95l-3.75-3.75L3 17.25zm17.7-10.04a1 1 0 0 0 0-1.42l-2.48-2.48a1 1 0 0 0-1.42 0l-1.95 1.95 3.75 3.75 2.1-2.09z"
                />
              </svg>
              Rename
            </button>
            <button
              class="title-edit-button"
              :disabled="!selectedConversationId"
              title="Archive chat (Shift+click to delete)"
              @click="archiveSelectedConversation($event)"
            >
              Archive
            </button>
          </template>
          <template v-else>
            <input
              ref="titleInputEl"
              v-model="renameDraft"
              class="chat-title-input"
              :disabled="isRenaming"
              @blur="saveConversationTitle"
              @keydown.enter.prevent="saveConversationTitle"
              @keydown.esc.prevent="cancelConversationTitleEdit"
            />
          </template>
        </div>
      </header>
      <div ref="messagesEl" class="messages">
        <article
          v-for="message in displayMessages"
          :key="message.id"
          class="message"
          :class="[message.role, { error: message.hasError }]"
        >
          <details v-if="message.role === 'assistant' && message.thinking" class="message-thinking">
            <summary>Thinking</summary>
            <div class="message-markdown" v-html="renderThinkingMarkdown(message.thinking)" />
          </details>
          <strong class="message-role">{{ message.role }}</strong>
          <template v-if="message.role !== 'assistant'">
            <p>{{ displayUserMessage(message) }}</p>
            <div v-if="message.attachments?.length" class="message-attachments">
              <a
                v-if="!message.hasError"
                v-for="(attachment, index) in message.attachments"
                :key="`${attachment}-${index}`"
                class="message-attachment-chip"
                :href="attachmentDownloadUrl(message, index)"
                :download="attachment"
                :title="`Download ${attachment}`"
              >
                <span aria-hidden="true">📄</span>
                {{ attachment }}
              </a>
              <span
                v-else
                v-for="(attachment, index) in message.attachments"
                :key="`${attachment}-${index}`"
                class="message-attachment-chip"
                :title="attachment"
              >
                <span aria-hidden="true">📄</span>
                {{ attachment }}
              </span>
            </div>
          </template>
          <div
            v-else
            class="message-markdown"
            v-html="renderAssistantMarkdown(message.content)"
          />
        </article>
        <article v-if="shouldShowPendingAssistantPlaceholder" class="message assistant pending-response">
          <strong class="message-role">assistant</strong>
          <div class="loading-dots" aria-live="polite" aria-label="Assistant is generating a response">
            <span />
            <span />
            <span />
          </div>
        </article>
      </div>
      <p v-if="streamError" class="error">{{ streamError }}</p>
      <form class="composer" @submit.prevent="sendMessage">
        <div v-if="selectedFiles.length" class="file-list">
          <span v-for="(file, index) in selectedFiles" :key="`${file.name}-${index}`" class="file-chip">
            {{ file.name }}
            <button type="button" class="file-chip-remove" @click="removeSelectedFile(index)">x</button>
          </span>
        </div>
        <button
          type="button"
          class="file-picker-button"
          aria-label="Upload files"
          title="Upload files"
          @click="openFilePicker"
        >
          +
        </button>
        <input
          ref="fileInputEl"
          class="file-picker-hidden"
          type="file"
          multiple
          accept="image/*,.pdf,.txt,.md,.markdown,.json,.csv,.xml,.yaml,.yml"
          @change="handleFileSelection"
        />
        <input v-model="draft" placeholder="Ask something..." />
        <button
          :type="isSending ? 'button' : 'submit'"
          :disabled="!isSending && !draft.trim()"
          class="composer-send-button"
          :class="{ 'stop-button': isSending }"
          @click="isSending ? stopGeneration() : undefined"
        >
          {{ isSending ? 'Stop' : 'Send' }}
        </button>
      </form>
    </section>
  <div v-if="showSettings" class="settings-overlay" @click.self="showSettings = false">
    <div class="settings-panel">
      <div class="settings-header">
        <h2 class="settings-title">Settings</h2>
        <button class="settings-close" @click="showSettings = false">✕</button>
      </div>
      <div class="settings-body">
        <label class="settings-label">LLM URL</label>
        <input class="settings-input" v-model="settingsForm.llm_url" placeholder="http://localhost:11434/v1" />
        <label class="settings-label">Model</label>
        <input class="settings-input" v-model="settingsForm.llm_model" placeholder="gpt-4o-mini" />
        <label class="settings-label">System Prompt</label>
        <textarea
          class="settings-textarea"
          v-model="settingsForm.system_prompt"
          placeholder="You are a helpful assistant."
          rows="6"
        />
        <p v-if="settingsError" class="settings-error">{{ settingsError }}</p>
      </div>
      <div class="settings-footer">
        <button class="settings-cancel" @click="showSettings = false">Cancel</button>
        <button class="settings-save" :disabled="settingsSaving" @click="saveSettings">
          {{ settingsSaving ? 'Saving…' : 'Save' }}
        </button>
      </div>
    </div>
  </div>
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
}

.layout[data-theme='dark'] {
  --bg: #0b1220;
  --surface: #121a2b;
  --surface-soft: #1a2438;
  --text: #e6edf8;
  --muted: #8ea0bf;
  --border: #27324a;
  --primary: #4f7cff;
  --primary-strong: #3e66e5;
}

.layout[data-theme='light'] {
  --bg: #f3f6fc;
  --surface: #ffffff;
  --surface-soft: #f7f9ff;
  --text: #0f172a;
  --muted: #475569;
  --border: #d8e0f0;
  --primary: #3368ff;
  --primary-strong: #2553d8;
}

.sidebar {
  border-right: 1px solid var(--border);
  padding: 1rem;
  background: var(--surface);
  display: grid;
  grid-template-rows: auto 1fr;
  gap: 1rem;
  overflow: hidden;
}

.sidebar-actions {
  display: grid;
  gap: 0.6rem;
}

.new-chat {
  width: 100%;
  padding: 0.65rem 0.75rem;
  border-radius: 0.6rem;
  border: none;
  background: var(--primary);
  color: #fff;
  font-weight: 600;
  cursor: pointer;
}

.sidebar-controls {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 0.4rem;
}

.control-btn {
  padding: 0.5rem 0;
  border-radius: 0.6rem;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
  cursor: pointer;
  font-size: 1rem;
  text-align: center;
}

.control-btn:hover {
  background: var(--surface);
}

.settings-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.settings-panel {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 1rem;
  width: min(480px, 90vw);
  display: grid;
  grid-template-rows: auto 1fr auto;
  max-height: 85vh;
  overflow: hidden;
}

.settings-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--border);
}

.settings-title {
  margin: 0;
  font-size: 1.1rem;
  font-weight: 700;
}

.settings-close {
  border: none;
  background: transparent;
  color: var(--muted);
  font-size: 1rem;
  cursor: pointer;
  padding: 0.25rem 0.5rem;
  border-radius: 0.4rem;
}

.settings-close:hover {
  color: var(--text);
  background: var(--surface-soft);
}

.settings-body {
  padding: 1.25rem;
  display: grid;
  gap: 0.5rem;
  overflow-y: auto;
}

.settings-label {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin-top: 0.4rem;
}

.settings-input,
.settings-textarea {
  padding: 0.6rem 0.75rem;
  border-radius: 0.55rem;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
  font-size: 0.9rem;
  font-family: inherit;
  width: 100%;
  box-sizing: border-box;
}

.settings-textarea {
  resize: vertical;
  min-height: 100px;
}

.settings-error {
  color: #ef4444;
  font-size: 0.85rem;
  margin: 0;
}

.settings-footer {
  display: flex;
  gap: 0.6rem;
  justify-content: flex-end;
  padding: 1rem 1.25rem;
  border-top: 1px solid var(--border);
}

.settings-cancel {
  padding: 0.55rem 1rem;
  border-radius: 0.6rem;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
  font-weight: 600;
  cursor: pointer;
}

.settings-save {
  padding: 0.55rem 1.1rem;
  border-radius: 0.6rem;
  border: none;
  background: var(--primary);
  color: #fff;
  font-weight: 600;
  cursor: pointer;
}

.settings-save:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.conversation-list {
  display: grid;
  gap: 0.32rem;
  overflow: auto;
  align-content: start;
}

.conversation-row {
  display: grid;
  grid-template-columns: 1fr auto auto;
  gap: 0.3rem;
}

.conversation-row.archived {
  grid-template-columns: 1fr auto auto;
}

.sidebar-generating-indicator {
  align-self: center;
  justify-self: center;
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 999px;
  background: var(--primary);
  box-shadow: 0 0 0 0 color-mix(in srgb, var(--primary) 60%, transparent);
  animation: sidebar-generating-pulse 1.4s ease-out infinite;
}

@keyframes sidebar-generating-pulse {
  0% {
    box-shadow: 0 0 0 0 color-mix(in srgb, var(--primary) 55%, transparent);
  }
  70% {
    box-shadow: 0 0 0 0.45rem color-mix(in srgb, var(--primary) 0%, transparent);
  }
  100% {
    box-shadow: 0 0 0 0 color-mix(in srgb, var(--primary) 0%, transparent);
  }
}

.conversation-row.active .conversation-item {
  border-color: var(--primary);
  background: color-mix(in srgb, var(--primary) 12%, var(--surface));
}

.archived-section {
  margin-top: 0.6rem;
  display: grid;
  gap: 0.3rem;
}

.archived-title {
  font-size: 0.75rem;
  color: var(--muted);
  text-transform: uppercase;
}

.conversation-item {
  text-align: left;
  padding: 0.38rem 0.5rem;
  border-radius: 0.45rem;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
  cursor: pointer;
  font-size: 0.84rem;
  line-height: 1.2;
  width: 100%;
  box-sizing: border-box;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.icon-button {
  border: 1px solid var(--border);
  border-radius: 0.45rem;
  background: var(--surface-soft);
  color: var(--text);
  cursor: pointer;
  padding: 0.1rem 0.35rem;
}

.chat-panel {
  display: grid;
  grid-template-rows: auto 1fr auto;
  overflow: hidden;
  background: var(--bg);
  position: relative;
}

.chat-header {
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--border);
}

.chat-title-wrap {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
}

.chat-title {
  margin: 0;
  font-size: 1.8rem;
  font-weight: 700;
}

.chat-title-input {
  font-size: 1.5rem;
  font-weight: 700;
  border: none;
  background: transparent;
  color: var(--text);
  outline: none;
  min-width: 220px;
  width: min(640px, 100%);
}

.title-edit-button {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
  border-radius: 0.6rem;
  padding: 0.4rem 0.65rem;
  font-size: 0.85rem;
  cursor: pointer;
}

.title-edit-button svg {
  width: 0.95rem;
  height: 0.95rem;
  fill: currentColor;
}

.messages {
  padding: 1rem 1.25rem 7rem;
  overflow: auto;
  display: grid;
  gap: 0.65rem;
  align-content: start;
}

.message {
  padding: 0.45rem 0.62rem;
  border-radius: 0.85rem;
  max-width: min(60%, 560px);
  line-height: 1.5;
  width: fit-content;
  transition: background-color 180ms ease, color 180ms ease, border-color 180ms ease;
}

.message.user {
  margin-left: auto;
  background: var(--primary);
  color: #fff;
}

.message.user.error {
  background: #f9df8b;
  color: #2f2411;
}

.message.user.error .message-attachment-chip {
  background: rgba(255, 255, 255, 0.65);
  border-color: rgba(47, 36, 17, 0.35);
  color: #2f2411;
}

.message.assistant {
  margin-right: auto;
  background: var(--surface);
  border: 1px solid var(--border);
}

.message.pending-response {
  min-width: 4.8rem;
}

.message-role {
  display: block;
  font-size: 0.67rem;
  text-transform: uppercase;
  opacity: 0.75;
  margin-bottom: 0.2rem;
}

.message p {
  margin: 0;
  font-size: 0.93rem;
}

.message-attachments {
  margin-top: 0.4rem;
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.message-attachment-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
  padding: 0.2rem 0.45rem;
  font-size: 0.75rem;
  color: inherit;
  text-decoration: none;
}

.message :deep(.message-markdown) {
  font-size: 0.93rem;
}

.message :deep(.message-markdown > :first-child) {
  margin-top: 0;
}

.message :deep(.message-markdown > :last-child) {
  margin-bottom: 0;
}

.message :deep(.message-markdown pre) {
  overflow-x: auto;
  padding: 0.6rem;
  border-radius: 0.5rem;
  background: var(--surface-soft);
}

.message :deep(.message-markdown code) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 0.88em;
}

.message :deep(.message-markdown p),
.message :deep(.message-markdown ul),
.message :deep(.message-markdown ol),
.message :deep(.message-markdown blockquote) {
  margin: 0.4rem 0;
}

.message :deep(.message-markdown a) {
  color: var(--primary);
}

.message-thinking {
  margin: 0 0 0.35rem;
  border: 1px solid var(--border);
  border-radius: 0.55rem;
  background: var(--surface-soft);
  padding: 0.35rem 0.5rem;
  font-size: 0.8rem;
}

.message-thinking summary {
  cursor: pointer;
  font-weight: 600;
}

.message-thinking p {
  margin-top: 0.35rem;
  white-space: pre-wrap;
}

.loading-dots {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  min-height: 1rem;
}

.loading-dots span {
  width: 0.42rem;
  height: 0.42rem;
  border-radius: 999px;
  background: color-mix(in srgb, var(--text) 72%, transparent);
  animation: loading-dot-bounce 1s ease-in-out infinite;
}

.loading-dots span:nth-child(2) {
  animation-delay: 0.12s;
}

.loading-dots span:nth-child(3) {
  animation-delay: 0.24s;
}

@keyframes loading-dot-bounce {
  0%,
  80%,
  100% {
    transform: translateY(0);
    opacity: 0.35;
  }
  40% {
    transform: translateY(-0.2rem);
    opacity: 1;
  }
}

.composer {
  position: absolute;
  left: 1.25rem;
  right: 1.25rem;
  bottom: 0.9rem;
  padding: 0.65rem;
  display: grid;
  grid-template-columns: auto 1fr auto;
  grid-template-rows: auto auto;
  grid-template-areas:
    'files files files'
    'upload input send';
  gap: 0.6rem;
  border: 1px solid var(--border);
  border-radius: 0.85rem;
  background: var(--surface);
  box-shadow: 0 12px 28px rgba(2, 10, 30, 0.22);
}

.file-list {
  grid-area: files;
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.file-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
  padding: 0.2rem 0.45rem;
  font-size: 0.75rem;
}

.file-chip-remove {
  border: none;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
}

.file-picker-button {
  grid-area: upload;
  width: 2.6rem;
  min-width: 2.6rem;
  padding: 0.75rem 0;
}

.file-picker-hidden {
  display: none;
}

.stop-button {
  background: #c2410c;
}

.composer input {
  grid-area: input;
  padding: 0.75rem 0.85rem;
  border-radius: 0.6rem;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
}

.composer button {
  padding: 0.75rem 1rem;
  border-radius: 0.6rem;
  border: none;
  background: var(--primary);
  color: #fff;
  font-weight: 600;
  cursor: pointer;
}

.composer-send-button {
  grid-area: send;
}

.composer button:disabled {
  opacity: 0.6;
  background: var(--primary-strong);
}

.error {
  color: #ef4444;
  padding: 0 1.25rem 0.5rem;
}
</style>
