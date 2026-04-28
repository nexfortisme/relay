<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import type { Conversation } from "../lib/api";
import AppIcon from "./AppIcon.vue";

const props = defineProps<{
  conversations: Conversation[];
  generatingConversationId: string | null;
  selectedConversationId: string | null;
  showArchived: boolean;
  theme: "dark" | "light";
}>();

defineEmits<{
  archive: [conversationId: string, event: MouseEvent];
  create: [];
  delete: [conversationId: string];
  openSettings: [];
  restore: [conversationId: string];
  select: [conversationId: string];
  toggleArchived: [];
  toggleTheme: [];
}>();

const activeConversations = computed(() =>
  props.conversations.filter((conversation) => !conversation.archived),
);
const archivedConversations = computed(() =>
  props.conversations.filter((conversation) => conversation.archived),
);

const isShiftPressed = ref(false);
const hoveredArchiveConversationId = ref<string | null>(null);

const handleKeydown = (event: KeyboardEvent) => {
  if (event.key === "Shift") {
    isShiftPressed.value = true;
  }
};

const handleKeyup = (event: KeyboardEvent) => {
  if (event.key === "Shift") {
    isShiftPressed.value = false;
  }
};

const handleWindowBlur = () => {
  isShiftPressed.value = false;
};

onMounted(() => {
  window.addEventListener("keydown", handleKeydown);
  window.addEventListener("keyup", handleKeyup);
  window.addEventListener("blur", handleWindowBlur);
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", handleKeydown);
  window.removeEventListener("keyup", handleKeyup);
  window.removeEventListener("blur", handleWindowBlur);
});
</script>

<template>
  <aside class="sidebar">
    <div class="sidebar-actions">
      <button class="new-chat" @click="$emit('create')">
        <AppIcon name="plus" :size="17" />
        New Chat
      </button>
      <div class="sidebar-controls">
        <button
          class="control-btn"
          :title="showArchived ? 'Hide archived' : 'Show archived'"
          @click="$emit('toggleArchived')"
        >
          <AppIcon name="archive" />
        </button>
        <button
          class="control-btn"
          :title="theme === 'dark' ? 'Light mode' : 'Dark mode'"
          @click="$emit('toggleTheme')"
        >
          <AppIcon :name="theme === 'dark' ? 'sun' : 'moon'" />
        </button>
        <button class="control-btn" title="Settings" @click="$emit('openSettings')">
          <AppIcon name="settings" />
        </button>
      </div>
    </div>

    <div class="conversation-list">
      <div
        v-for="conversation in activeConversations"
        :key="conversation.id"
        class="conversation-row"
        :class="{ active: conversation.id === selectedConversationId }"
      >
        <button class="conversation-item" @click="$emit('select', conversation.id)">
          <span class="conversation-title">{{ conversation.title }}</span>
          <span
            v-if="generatingConversationId === conversation.id"
            class="sidebar-generating-indicator"
            aria-label="Generating response"
            title="Generating response"
          />
        </button>
        <button
          class="icon-button"
          :title="
            hoveredArchiveConversationId === conversation.id && isShiftPressed
              ? 'Delete chat'
              : 'Archive chat (Shift+click to delete)'
          "
          @mouseenter="hoveredArchiveConversationId = conversation.id"
          @mouseleave="hoveredArchiveConversationId = null"
          @click.stop="$emit('archive', conversation.id, $event)"
        >
          <AppIcon
            :name="
              hoveredArchiveConversationId === conversation.id && isShiftPressed
                ? 'trash'
                : 'archive'
            "
            :size="15"
          />
        </button>
      </div>

      <div class="archived-section">
        <div class="archived-title">Archived chats ({{ archivedConversations.length }})</div>
      </div>
      <div v-if="showArchived" class="archived-section">
        <div
          v-for="conversation in archivedConversations"
          :key="conversation.id"
          class="conversation-row archived"
        >
          <button class="conversation-item" @click="$emit('select', conversation.id)">
            <span class="conversation-title">{{ conversation.title }}</span>
          </button>
          <button
            class="icon-button"
            title="Restore chat"
            @click.stop="$emit('restore', conversation.id)"
          >
            <AppIcon name="restore" :size="15" />
          </button>
          <button
            class="icon-button danger"
            title="Delete chat"
            @click.stop="$emit('delete', conversation.id)"
          >
            <AppIcon name="trash" :size="15" />
          </button>
        </div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  border-right: 1px solid var(--border);
  padding: 1rem;
  background: var(--sidebar);
  display: grid;
  grid-template-rows: auto 1fr;
  gap: 1rem;
  overflow: hidden;
}

.sidebar-actions {
  display: grid;
  gap: 0.7rem;
}

.new-chat {
  width: 100%;
  min-height: 2.55rem;
  padding: 0.65rem 0.85rem;
  border-radius: 0.5rem;
  border: 1px solid color-mix(in srgb, var(--primary) 45%, transparent);
  background: var(--primary);
  color: #fff;
  font-weight: 650;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.45rem;
}

.new-chat:hover {
  background: var(--primary-strong);
}

.sidebar-controls {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 0.45rem;
}

.control-btn,
.icon-button {
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--muted);
  cursor: pointer;
  display: inline-grid;
  place-items: center;
}

.control-btn {
  min-height: 2.2rem;
  border-radius: 0.5rem;
}

.control-btn:hover,
.icon-button:hover {
  border-color: color-mix(in srgb, var(--primary) 42%, var(--border));
  color: var(--text);
  background: var(--surface-hover);
}

.conversation-list {
  display: grid;
  gap: 0.3rem;
  overflow: auto;
  align-content: start;
}

.conversation-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 0.28rem;
  align-items: center;
}

.conversation-row.archived {
  grid-template-columns: minmax(0, 1fr) auto auto;
}

.conversation-row.active .conversation-item {
  border-color: color-mix(in srgb, var(--primary) 42%, var(--border));
  background: var(--selected);
  color: var(--text);
}

.conversation-item {
  text-align: left;
  min-height: 2.15rem;
  padding: 0.48rem 0.62rem;
  border-radius: 0.45rem;
  border: 1px solid transparent;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  font-size: 0.87rem;
  line-height: 1.2;
  width: 100%;
  box-sizing: border-box;
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.5rem;
}

.conversation-title {
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.conversation-item:hover {
  background: var(--surface);
  color: var(--text);
}

.icon-button {
  width: 2.15rem;
  height: 2.15rem;
  border-radius: 0.45rem;
}

.icon-button.danger:hover {
  border-color: color-mix(in srgb, var(--danger) 55%, var(--border));
  color: var(--danger);
}

.sidebar-generating-indicator {
  align-self: center;
  justify-self: center;
  width: 0.48rem;
  height: 0.48rem;
  border-radius: 999px;
  background: var(--primary);
  box-shadow: 0 0 0 0 color-mix(in srgb, var(--primary) 55%, transparent);
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

.archived-section {
  margin-top: 0.65rem;
  display: grid;
  gap: 0.3rem;
}

.archived-title {
  padding: 0.35rem 0.55rem 0.15rem;
  font-size: 0.72rem;
  color: var(--muted);
  text-transform: uppercase;
  font-weight: 700;
}
</style>
