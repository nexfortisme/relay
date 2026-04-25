<script setup lang="ts">
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { messageAttachmentDownloadUrl, type Message } from '../lib/api'
import type { DisplayMessage } from '../types'
import AppIcon from './AppIcon.vue'

const props = defineProps<{
  messages: DisplayMessage[]
  pendingAssistant: boolean
  requeueDisabled?: boolean
}>()

const emit = defineEmits<{
  requeue: [message: DisplayMessage]
}>()

const messagesEl = ref<HTMLElement | null>(null)
const copiedMessageId = ref<string | null>(null)
let copiedResetTimer: ReturnType<typeof setTimeout> | null = null

const previewSrc = ref<string | null>(null)
const previewFilename = ref<string>('')

const IMAGE_EXTENSIONS = /\.(png|jpe?g|gif|webp|svg|bmp|ico|avif|tiff?)$/i

function isImageFile(name: string): boolean {
  return IMAGE_EXTENSIONS.test(name)
}

function openPreview(src: string, filename: string) {
  previewSrc.value = src
  previewFilename.value = filename
}

function closePreview() {
  previewSrc.value = null
  previewFilename.value = ''
}

function handleMarkdownClick(event: MouseEvent) {
  const target = event.target
  if (!(target instanceof HTMLImageElement)) return
  event.preventDefault()
  openPreview(target.src, target.alt || 'image')
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && previewSrc.value) {
    closePreview()
  }
}

function handleImageAttachmentClick(event: MouseEvent, message: Message, index: number) {
  event.preventDefault()
  const src = attachmentDownloadUrl(message, index)
  const filename = message.attachments?.[index] ?? 'image'
  openPreview(src, filename)
}

marked.setOptions({
  gfm: true,
  breaks: true,
})

watch(
  () => {
    const latestMessage = props.messages[props.messages.length - 1]
    return [props.messages.length, props.pendingAssistant, latestMessage?.content, latestMessage?.thinking]
  },
  () => {
    void scrollToBottom()
  },
  { flush: 'post' },
)

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

function formatElapsed(ms: number | undefined): string {
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

function renderMarkdown(content: string): string {
  const parsed = marked.parse(content, { async: false })
  return DOMPurify.sanitize(parsed)
}

function attachmentDownloadUrl(message: Message, attachmentIndex: number): string {
  return messageAttachmentDownloadUrl(message.conversationId, message.id, attachmentIndex)
}

function copyTextForMessage(message: DisplayMessage): string {
  if (message.role === 'assistant') {
    return message.content
  }
  return displayUserMessage(message)
}

async function copyMessage(message: DisplayMessage) {
  const text = copyTextForMessage(message)
  if (!text) {
    return
  }
  await writeClipboardText(text)
  copiedMessageId.value = message.id
  if (copiedResetTimer) {
    clearTimeout(copiedResetTimer)
  }
  copiedResetTimer = setTimeout(() => {
    copiedMessageId.value = null
    copiedResetTimer = null
  }, 1600)
}

async function writeClipboardText(text: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text)
    return
  }

  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.left = '-9999px'
  document.body.appendChild(textarea)
  textarea.select()
  document.execCommand('copy')
  document.body.removeChild(textarea)
}

function handleThinkingPanelClick(event: MouseEvent) {
  const details = event.currentTarget
  if (!(details instanceof HTMLDetailsElement) || !details.open) {
    return
  }
  const target = event.target
  if (target instanceof Element && target.closest('summary')) {
    return
  }
  details.open = false
}

async function scrollToBottom() {
  await nextTick()
  const el = messagesEl.value
  if (!el) {
    return
  }
  el.scrollTop = el.scrollHeight
}

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  if (copiedResetTimer) {
    clearTimeout(copiedResetTimer)
  }
  document.removeEventListener('keydown', handleKeydown)
})

defineExpose({ scrollToBottom })
</script>

<template>
  <div ref="messagesEl" class="messages">
    <article
      v-for="message in messages"
      :key="message.id"
      class="message"
      :class="[message.role, { error: message.hasError }]"
    >
      <details
        v-if="message.role === 'assistant' && message.thinking"
        class="message-thinking"
        @click="handleThinkingPanelClick"
      >
        <summary>Thinking</summary>
        <div class="message-markdown" v-html="renderMarkdown(message.thinking)" />
      </details>
      <strong class="message-role">{{ message.role }}</strong>
      <template v-if="message.role !== 'assistant'">
        <p>{{ displayUserMessage(message) }}</p>
        <div v-if="message.attachments?.length" class="message-attachments">
          <template v-if="!message.hasError">
            <template v-for="(attachment, index) in message.attachments" :key="`${attachment}-${index}`">
              <button
                v-if="isImageFile(attachment)"
                type="button"
                class="message-attachment-chip message-attachment-chip--image"
                :title="`Preview ${attachment}`"
                @click="handleImageAttachmentClick($event, message, index)"
              >
                <img
                  class="message-attachment-thumb"
                  :src="attachmentDownloadUrl(message, index)"
                  :alt="attachment"
                />
                <span class="message-attachment-name">{{ attachment }}</span>
              </button>
              <a
                v-else
                class="message-attachment-chip"
                :href="attachmentDownloadUrl(message, index)"
                :download="attachment"
                :title="`Download ${attachment}`"
              >
                <AppIcon name="file" :size="14" />
                {{ attachment }}
              </a>
            </template>
          </template>
          <template v-else>
            <span
              v-for="(attachment, index) in message.attachments"
              :key="`${attachment}-${index}`"
              class="message-attachment-chip"
              :title="attachment"
            >
              <AppIcon name="file" :size="14" />
              {{ attachment }}
            </span>
          </template>
        </div>
      </template>
      <div v-else class="message-markdown" v-html="renderMarkdown(message.content)" @click="handleMarkdownClick" />
      <div
        v-if="message.role === 'assistant' && typeof message.elapsedMs === 'number' && message.elapsedMs > 0"
        class="message-elapsed"
        :title="`Generated in ${formatElapsed(message.elapsedMs)}`"
      >
        <AppIcon name="clock" :size="12" />
        {{ formatElapsed(message.elapsedMs) }}
      </div>
      <div class="message-actions">
        <button
          v-if="message.role === 'user'"
          type="button"
          class="message-action-button"
          title="Requeue message"
          aria-label="Requeue message"
          :disabled="requeueDisabled"
          @click="emit('requeue', message)"
        >
          <AppIcon name="refresh" :size="14" />
        </button>
        <button
          type="button"
          class="message-action-button"
          :class="{ copied: copiedMessageId === message.id }"
          :title="copiedMessageId === message.id ? 'Copied' : 'Copy message'"
          :aria-label="copiedMessageId === message.id ? 'Copied' : 'Copy message'"
          @click="copyMessage(message)"
        >
          <AppIcon :name="copiedMessageId === message.id ? 'check' : 'copy'" :size="14" />
        </button>
      </div>
    </article>
    <article v-if="pendingAssistant" class="message assistant pending-response">
      <strong class="message-role">assistant</strong>
      <div class="loading-dots" aria-live="polite" aria-label="Assistant is generating a response">
        <span />
        <span />
        <span />
      </div>
    </article>
  </div>

  <Teleport to="body">
    <div v-if="previewSrc" class="image-preview-overlay" @click.self="closePreview">
      <div class="image-preview-dialog" role="dialog" aria-modal="true" aria-label="Image preview">
        <div class="image-preview-toolbar">
          <a
            class="image-preview-btn"
            :href="previewSrc"
            :download="previewFilename"
            title="Download image"
            aria-label="Download image"
          >
            <AppIcon name="download" :size="17" />
          </a>
          <button
            type="button"
            class="image-preview-btn"
            title="Close preview"
            aria-label="Close preview"
            @click="closePreview"
          >
            <AppIcon name="x" :size="17" />
          </button>
        </div>
        <div class="image-preview-body">
          <img class="image-preview-img" :src="previewSrc" :alt="previewFilename" />
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.messages {
  padding: 1.1rem 1.35rem 7.1rem;
  overflow: auto;
  display: grid;
  gap: 0.68rem;
  align-content: start;
}

.message {
  padding: 0.58rem 0.72rem;
  border-radius: 0.75rem;
  max-width: min(68%, 660px);
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
  opacity: 0.72;
  margin-bottom: 0.24rem;
  font-weight: 800;
}

.message p {
  margin: 0;
  font-size: 0.94rem;
}

.message-attachments {
  margin-top: 0.45rem;
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.message-elapsed {
  display: inline-flex;
  align-items: center;
  gap: 0.28rem;
  margin-top: 0.36rem;
  font-size: 0.72rem;
  opacity: 0.62;
  font-variant-numeric: tabular-nums;
}

.message-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.35rem;
  margin-top: 0.44rem;
  min-height: 1.75rem;
}

.message.assistant .message-actions {
  justify-content: flex-start;
}

.message-action-button {
  width: 1.75rem;
  height: 1.75rem;
  border: 1px solid color-mix(in srgb, currentColor 24%, transparent);
  border-radius: 0.45rem;
  background: color-mix(in srgb, var(--surface) 54%, transparent);
  color: inherit;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  opacity: 0.76;
  transition: background-color 160ms ease, border-color 160ms ease, opacity 160ms ease;
}

.message-action-button:hover,
.message-action-button:focus-visible {
  opacity: 1;
  border-color: color-mix(in srgb, currentColor 38%, transparent);
  background: color-mix(in srgb, var(--surface) 72%, transparent);
}

.message-action-button:disabled {
  cursor: not-allowed;
  opacity: 0.38;
}

.message-action-button.copied {
  opacity: 1;
}

.message-attachment-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface) 76%, transparent);
  color: inherit;
  padding: 0.22rem 0.48rem;
  font-size: 0.75rem;
  text-decoration: none;
  max-width: 18rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.message-attachment-chip--image {
  cursor: pointer;
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface) 76%, transparent);
  color: inherit;
  padding: 0;
  overflow: hidden;
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  border-radius: 999px;
  font-size: 0.75rem;
  max-width: 18rem;
}

.message-attachment-chip--image:hover {
  border-color: var(--primary);
}

.message-attachment-thumb {
  width: 1.8rem;
  height: 1.8rem;
  object-fit: cover;
  flex: 0 0 auto;
  border-radius: 999px 0 0 999px;
}

.message-attachment-name {
  padding-right: 0.48rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.message-markdown :deep(img) {
  max-width: 100%;
  border-radius: 0.45rem;
  cursor: zoom-in;
  display: block;
}

.image-preview-overlay {
  position: fixed;
  inset: 0;
  background: rgba(4, 9, 20, 0.82);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 200;
  padding: 1.5rem;
}

.image-preview-dialog {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 0.75rem;
  box-shadow: var(--shadow);
  display: flex;
  flex-direction: column;
  max-width: min(90vw, 1000px);
  max-height: 90vh;
  overflow: hidden;
}

.image-preview-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.5rem 0.6rem;
  border-bottom: 1px solid var(--border);
  flex: 0 0 auto;
}

.image-preview-btn {
  width: 2rem;
  height: 2rem;
  border: 1px solid transparent;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  border-radius: 0.45rem;
  display: inline-grid;
  place-items: center;
  text-decoration: none;
}

.image-preview-btn:hover {
  color: var(--text);
  background: var(--surface-hover);
  border-color: var(--border);
}

.image-preview-body {
  overflow: auto;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
}

.image-preview-img {
  max-width: 100%;
  max-height: calc(90vh - 6rem);
  object-fit: contain;
  border-radius: 0.35rem;
  display: block;
}

.message-markdown {
  font-size: 0.94rem;
}

.message-markdown :deep(> :first-child) {
  margin-top: 0;
}

.message-markdown :deep(> :last-child) {
  margin-bottom: 0;
}

.message-markdown :deep(pre) {
  overflow-x: auto;
  padding: 0.72rem;
  border-radius: 0.5rem;
  background: var(--surface-soft);
}

.message-markdown :deep(code) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 0.88em;
}

.message-markdown :deep(p),
.message-markdown :deep(ul),
.message-markdown :deep(ol),
.message-markdown :deep(blockquote) {
  margin: 0.45rem 0;
}

.message-markdown :deep(a) {
  color: var(--primary);
}

.message-thinking {
  margin: 0 0 0.4rem;
  border: 1px solid var(--border);
  border-radius: 0.55rem;
  background: var(--surface-soft);
  padding: 0.4rem 0.52rem;
  font-size: 0.8rem;
}

.message-thinking summary {
  cursor: pointer;
  font-weight: 650;
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
</style>
