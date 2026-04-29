<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { storeToRefs } from 'pinia'
import ChatComposer from './components/ChatComposer.vue'
import ChatHeader from './components/ChatHeader.vue'
import ConversationSidebar from './components/ConversationSidebar.vue'
import EmptyChatGreeting from './components/EmptyChatGreeting.vue'
import MessageList from './components/MessageList.vue'
import SettingsModal from './components/SettingsModal.vue'
import { DEFAULT_CONVERSATION_TITLE, useAppStore } from './stores/appStore'

const appStore = useAppStore()
const {
  conversations,
  conversationTokenCount,
  draft,
  generatingConversationId,
  isConversationTokenCapReached,
  isEditingTitle,
  isRenaming,
  isSending,
  isSuggestingTitle,
  messages,
  renameDraft,
  selectedConversation,
  selectedConversationId,
  selectedFiles,
  settingsError,
  settingsForm,
  settingsSaving,
  shouldShowPendingAssistantPlaceholder,
  showArchived,
  showSettings,
  streamError,
  theme,
} = storeToRefs(appStore)

const shouldShowEmptyGreeting = computed(
  () => messages.value.length === 0 && !shouldShowPendingAssistantPlaceholder.value,
)

onMounted(appStore.initializeApp)
onUnmounted(appStore.closeStream)
</script>

<template>
  <main class="layout" :data-theme="theme">
    <ConversationSidebar
      :conversations="conversations"
      :generating-conversation-id="generatingConversationId"
      :selected-conversation-id="selectedConversationId"
      :show-archived="showArchived"
      :theme="theme"
      @archive="appStore.archiveChat"
      @create="appStore.handleCreateConversation"
      @delete="appStore.deleteChat"
      @open-settings="appStore.openSettings"
      @restore="appStore.restoreChat"
      @select="appStore.selectConversation"
      @toggle-archived="appStore.toggleArchived"
      @toggle-theme="appStore.toggleTheme"
    />

    <section class="chat-panel">
      <ChatHeader
        v-model:rename-draft="renameDraft"
        :is-editing="isEditingTitle"
        :is-renaming="isRenaming"
        :is-suggesting-title="isSuggestingTitle"
        :selected-conversation-id="selectedConversationId"
        :title="selectedConversation?.title ?? DEFAULT_CONVERSATION_TITLE"
        @archive="appStore.archiveSelectedConversation"
        @begin-edit="appStore.beginConversationTitleEdit"
        @cancel-edit="appStore.cancelConversationTitleEdit"
        @save-title="appStore.saveConversationTitle"
        @suggest-title="appStore.suggestConversationTitleWithLLM"
      />
      <MessageList
        :messages="messages"
        :token-count="conversationTokenCount"
        :max-token-count="appStore.maxConversationTokenCount"
        :pending-assistant="shouldShowPendingAssistantPlaceholder"
        :requeue-disabled="isSending"
        :theme="theme"
        @requeue="appStore.handleRequeueMessage"
      />
      <p v-if="streamError" class="error">{{ streamError }}</p>
      <EmptyChatGreeting
        v-if="shouldShowEmptyGreeting"
        :class="{ 'empty-chat-greeting--with-files': selectedFiles.length > 0 }"
      />
      <ChatComposer
        :draft="draft"
        :is-sending="isSending"
        :token-limit-reached="isConversationTokenCapReached"
        :selected-files="selectedFiles"
        @remove-file="appStore.removeSelectedFile"
        @send="appStore.sendMessage"
        @stop="appStore.stopGeneration"
        @update-draft="appStore.setDraft"
        @update-files="appStore.handleSelectedFiles"
      />
    </section>
    <SettingsModal
      v-if="showSettings"
      v-model:settings="settingsForm"
      :error="settingsError"
      :saving="settingsSaving"
      @close="appStore.closeSettings"
      @save="appStore.saveSettings"
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
    Inter,
    ui-sans-serif,
    system-ui,
    -apple-system,
    BlinkMacSystemFont,
    'Segoe UI',
    sans-serif;
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
