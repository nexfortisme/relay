<script setup lang="ts">
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { nextTick, onUnmounted, ref, watch } from 'vue'
import { messageAttachmentDownloadUrl, type Message } from '../lib/api'
import type { DisplayMessage } from '../types'
import AppIcon from './AppIcon.vue'

const props = defineProps<{
  messages: DisplayMessage[]
  pendingAssistant: boolean
}>()

const messagesEl = ref<HTMLElement | null>(null)
const copiedMessageId = ref<string | null>(null)
let copiedResetTimer: ReturnType<typeof setTimeout> | null = null

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

onUnmounted(() => {
  if (copiedResetTimer) {
    clearTimeout(copiedResetTimer)
  }
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
            <a
              v-for="(attachment, index) in message.attachments"
              :key="`${attachment}-${index}`"
              class="message-attachment-chip"
              :href="attachmentDownloadUrl(message, index)"
              :download="attachment"
              :title="`Download ${attachment}`"
            >
              <AppIcon name="file" :size="14" />
              {{ attachment }}
            </a>
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
      <div v-else class="message-markdown" v-html="renderMarkdown(message.content)" />
      <div class="message-actions">
        <button
          type="button"
          class="message-copy-button"
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

.message-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 0.44rem;
  min-height: 1.75rem;
}

.message.assistant .message-actions {
  justify-content: flex-start;
}

.message-copy-button {
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

.message-copy-button:hover,
.message-copy-button:focus-visible {
  opacity: 1;
  border-color: color-mix(in srgb, currentColor 38%, transparent);
  background: color-mix(in srgb, var(--surface) 72%, transparent);
}

.message-copy-button.copied {
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
