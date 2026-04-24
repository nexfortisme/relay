<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import AppIcon from './AppIcon.vue'

const props = defineProps<{
  isEditing: boolean
  isRenaming: boolean
  isSuggestingTitle: boolean
  renameDraft: string
  selectedConversationId: string | null
  title: string
}>()

defineEmits<{
  archive: [event: MouseEvent]
  beginEdit: []
  cancelEdit: []
  saveTitle: []
  suggestTitle: []
  'update:renameDraft': [value: string]
}>()

const titleInputEl = ref<HTMLInputElement | null>(null)

watch(
  () => props.isEditing,
  async (isEditing) => {
    if (!isEditing) {
      return
    }
    await nextTick()
    titleInputEl.value?.focus()
    titleInputEl.value?.select()
  },
)
</script>

<template>
  <header class="chat-header">
    <div v-if="!isEditing" class="title-line">
      <div class="title-group">
        <h1 class="chat-title" :class="{ 'chat-title-suggesting': isSuggestingTitle }">{{ title }}</h1>
        <button
          class="title-icon-button"
          :disabled="isRenaming || isSuggestingTitle"
          title="Rename chat"
          aria-label="Rename chat"
          @click="$emit('beginEdit')"
        >
          <AppIcon name="pencil" :size="16" />
        </button>
        <button
          class="title-icon-button"
          :disabled="!selectedConversationId || isRenaming || isSuggestingTitle"
          title="Suggest title with AI"
          aria-label="Suggest title with AI"
          @click="$emit('suggestTitle')"
        >
          <AppIcon name="sparkles" :size="16" />
        </button>
      </div>
      <button
        class="header-action"
        :disabled="!selectedConversationId"
        title="Archive chat (Shift+click to delete)"
        @click="$emit('archive', $event)"
      >
        <AppIcon name="archive" :size="16" />
        Archive
      </button>
    </div>

    <div v-else class="title-edit-line">
      <input
        ref="titleInputEl"
        class="chat-title-input"
        :value="renameDraft"
        :disabled="isRenaming"
        @input="$emit('update:renameDraft', ($event.target as HTMLInputElement).value)"
        @blur="$emit('saveTitle')"
        @keydown.enter.prevent="$emit('saveTitle')"
        @keydown.esc.prevent="$emit('cancelEdit')"
      />
      <button class="title-icon-button" :disabled="isRenaming" title="Save title" @mousedown.prevent="$emit('saveTitle')">
        <AppIcon name="check" :size="16" />
      </button>
      <button class="title-icon-button" title="Cancel rename" @mousedown.prevent="$emit('cancelEdit')">
        <AppIcon name="x" :size="16" />
      </button>
    </div>
  </header>
</template>

<style scoped>
.chat-header {
  padding: 1rem 1.35rem;
  border-bottom: 1px solid var(--border);
  background: color-mix(in srgb, var(--bg) 88%, var(--surface));
}

.title-line,
.title-edit-line {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  min-height: 2.45rem;
}

.title-group {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 0.42rem;
}

.chat-title {
  margin: 0;
  font-size: 1.22rem;
  font-weight: 720;
  line-height: 1.2;
  max-width: min(58vw, 760px);
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.chat-title-suggesting {
  background: linear-gradient(
    110deg,
    color-mix(in srgb, var(--text) 70%, #fff) 5%,
    color-mix(in srgb, var(--primary) 60%, #fff) 35%,
    #fff 50%,
    color-mix(in srgb, var(--primary) 60%, #fff) 65%,
    color-mix(in srgb, var(--text) 70%, #fff) 95%
  );
  background-size: 260% 100%;
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
  filter: drop-shadow(0 0 0.4rem color-mix(in srgb, var(--primary) 35%, transparent));
  animation: title-shimmer 1s linear infinite;
}

@keyframes title-shimmer {
  0% {
    background-position: 200% 0;
  }
  100% {
    background-position: -20% 0;
  }
}

.title-icon-button,
.header-action {
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--muted);
  cursor: pointer;
  border-radius: 0.45rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.title-icon-button {
  width: 2rem;
  height: 2rem;
  flex: 0 0 auto;
}

.header-action {
  min-height: 2.2rem;
  gap: 0.42rem;
  padding: 0 0.72rem;
  font-weight: 650;
}

.title-icon-button:hover,
.header-action:hover {
  color: var(--text);
  border-color: color-mix(in srgb, var(--primary) 42%, var(--border));
  background: var(--surface-hover);
}

.title-icon-button:disabled,
.header-action:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.title-edit-line {
  justify-content: flex-start;
}

.chat-title-input {
  font-size: 1.18rem;
  font-weight: 720;
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  background: var(--surface);
  color: var(--text);
  outline: none;
  min-width: 220px;
  width: min(640px, 100%);
  padding: 0.48rem 0.62rem;
}

.chat-title-input:focus {
  border-color: var(--primary);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 18%, transparent);
}
</style>
