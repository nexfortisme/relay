<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import ChatComposer from '../components/ChatComposer.vue'
import ChatHeader from '../components/ChatHeader.vue'
import ConversationSidebar from '../components/ConversationSidebar.vue'
import EmptyChatGreeting from '../components/EmptyChatGreeting.vue'
import MessageList from '../components/MessageList.vue'
import PageNavTabs from '../components/PageNavTabs.vue'
import { DEFAULT_CONVERSATION_TITLE, useConversationStore } from '../stores/conversationStore'
import { useChatStore } from '../stores/chatStore'
import { useUiStore } from '../stores/uiStore'

const conversationStore = useConversationStore()
const chatStore = useChatStore()
const uiStore = useUiStore()
const router = useRouter()

const {
  conversations,
  isEditingTitle,
  isRenaming,
  isSuggestingTitle,
  renameDraft,
  selectedConversation,
  selectedConversationId,
  showArchived,
} = storeToRefs(conversationStore)

const {
  conversationTokenCount,
  draft,
  generatingConversationId,
  isConversationTokenCapReached,
  isSelectedConversationWaitingForAssistant,
  isSending,
  messages,
  selectedFiles,
  streamError,
} = storeToRefs(chatStore)

const { isSidebarCollapsed, theme } = storeToRefs(uiStore)

const shouldShowEmptyGreeting = computed(
  () => messages.value.length === 0 && !isSelectedConversationWaitingForAssistant.value,
)
const chatModels = computed(() => {
  const seen = new Set<string>()
  const models: string[] = []
  for (const message of messages.value) {
    if (message.role !== 'assistant') {
      continue
    }
    const model = message.model?.trim()
    if (!model || seen.has(model)) {
      continue
    }
    seen.add(model)
    models.push(model)
  }
  return models
})

const goHome = () => {
  router.push('/home')
}

onMounted(chatStore.resumeSelectedConversationStream)
</script>

<template>
  <div class="chat-view" :class="{ 'chat-view--sidebar-collapsed': isSidebarCollapsed }">
    <ConversationSidebar
      v-if="!isSidebarCollapsed"
      :conversations="conversations"
      :generating-conversation-id="generatingConversationId"
      :selected-conversation-id="selectedConversationId"
      :show-archived="showArchived"
      @archive="chatStore.archiveChat"
      @create="chatStore.handleCreateConversation"
      @home="goHome"
      @delete="chatStore.deleteChat"
      @restore="chatStore.restoreChat"
      @select="chatStore.selectConversation"
      @toggle-archived="conversationStore.toggleArchived"
      @toggle-collapse="uiStore.toggleSidebarCollapsed"
      @toggle-favorite="chatStore.toggleFavoriteChat"
    />
    <button
      v-if="!isSidebarCollapsed"
      class="mobile-sidebar-backdrop"
      type="button"
      aria-label="Close sidebar"
      @click="uiStore.toggleSidebarCollapsed"
    />

    <section class="chat-panel">
      <PageNavTabs />
      <ChatHeader
        v-model:rename-draft="renameDraft"
        :is-editing="isEditingTitle"
        :is-generating="generatingConversationId === selectedConversationId"
        :is-renaming="isRenaming"
        :is-suggesting-title="isSuggestingTitle"
        :selected-conversation-id="selectedConversationId"
        :title="selectedConversation?.title ?? DEFAULT_CONVERSATION_TITLE"
        :models="chatModels"
        :token-count="conversationTokenCount"
        :max-token-count="chatStore.maxConversationTokenCount"
        :is-favorite="selectedConversation?.favorite ?? false"
        @archive="chatStore.archiveSelectedConversation"
        @begin-edit="conversationStore.beginConversationTitleEdit"
        @cancel-edit="conversationStore.cancelConversationTitleEdit"
        @save-title="conversationStore.saveConversationTitle"
        @suggest-title="conversationStore.suggestConversationTitleWithLLM"
        @toggle-favorite="
          selectedConversationId && chatStore.toggleFavoriteChat(selectedConversationId)
        "
      />
      <MessageList
        :messages="messages"
        :pending-assistant="isSelectedConversationWaitingForAssistant"
        :requeue-disabled="isSending"
        :theme="theme"
        @requeue="chatStore.handleRequeueMessage"
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
        @remove-file="chatStore.removeSelectedFile"
        @send="chatStore.sendMessage"
        @stop="chatStore.stopGeneration"
        @update-draft="chatStore.setDraft"
        @update-files="chatStore.handleSelectedFiles"
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
  position: relative;
}

.chat-view.chat-view--sidebar-collapsed {
  grid-template-columns: 1fr;
}

.chat-panel {
  display: grid;
  grid-template-rows: auto auto 1fr auto;
  min-width: 0;
  overflow: hidden;
  background: var(--bg);
  position: relative;
}

.error {
  color: #ef4444;
  padding: 0 1.25rem 0.5rem;
}

.mobile-sidebar-backdrop {
  display: none;
}

@media (max-width: 760px) {
  .chat-view {
    grid-template-columns: 1fr;
  }

  .chat-view:not(.chat-view--sidebar-collapsed) :deep(.sidebar) {
    position: absolute;
    inset: 0 auto 0 0;
    width: min(20rem, 86vw);
    z-index: 40;
    box-sizing: border-box;
    box-shadow: var(--shadow);
  }

  .mobile-sidebar-backdrop {
    position: absolute;
    inset: 0;
    z-index: 30;
    display: block;
    border: 0;
    background: rgba(4, 9, 20, 0.52);
    cursor: pointer;
  }

  .error {
    padding-inline: 0.9rem;
  }
}
</style>
