<script setup lang="ts">
import { computed, onUnmounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import ChatComposer from '../components/ChatComposer.vue'
import ChatHeader from '../components/ChatHeader.vue'
import ConversationSidebar from '../components/ConversationSidebar.vue'
import EmptyChatGreeting from '../components/EmptyChatGreeting.vue'
import MessageList from '../components/MessageList.vue'
import PageNavTabs from '../components/PageNavTabs.vue'
import { DEFAULT_CONVERSATION_TITLE, useAppStore } from '../stores/appStore'

const appStore = useAppStore()
const router = useRouter()
const {
  conversations,
  conversationTokenCount,
  draft,
  generatingConversationId,
  isConversationTokenCapReached,
  isEditingTitle,
  isRenaming,
  isSending,
  isSidebarCollapsed,
  isSuggestingTitle,
  messages,
  renameDraft,
  selectedConversation,
  selectedConversationId,
  selectedFiles,
  shouldShowPendingAssistantPlaceholder,
  showArchived,
  streamError,
  theme,
} = storeToRefs(appStore)

const shouldShowEmptyGreeting = computed(
  () => messages.value.length === 0 && !shouldShowPendingAssistantPlaceholder.value,
)

const goHome = () => {
  router.push('/')
}

onUnmounted(appStore.closeStream)
</script>

<template>
  <div class="chat-view" :class="{ 'chat-view--sidebar-collapsed': isSidebarCollapsed }">
    <ConversationSidebar
      v-if="!isSidebarCollapsed"
      :conversations="conversations"
      :generating-conversation-id="generatingConversationId"
      :selected-conversation-id="selectedConversationId"
      :show-archived="showArchived"
      :theme="theme"
      @archive="appStore.archiveChat"
      @create="appStore.handleCreateConversation"
      @home="goHome"
      @delete="appStore.deleteChat"
      @restore="appStore.restoreChat"
      @select="appStore.selectConversation"
      @toggle-archived="appStore.toggleArchived"
      @toggle-collapse="appStore.toggleSidebarCollapsed"
      @toggle-theme="appStore.toggleTheme"
    />

    <section class="chat-panel">
      <PageNavTabs />
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
  </div>
</template>

<style scoped>
.chat-view {
  display: grid;
  grid-template-columns: 280px 1fr;
  height: 100%;
  overflow: hidden;
}

.chat-view.chat-view--sidebar-collapsed {
  grid-template-columns: 1fr;
}

.chat-panel {
  display: grid;
  grid-template-rows: auto auto 1fr auto;
  overflow: hidden;
  background: var(--bg);
  position: relative;
}

.error {
  color: #ef4444;
  padding: 0 1.25rem 0.5rem;
}
</style>
