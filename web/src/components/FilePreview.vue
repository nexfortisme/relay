<script setup lang="ts">
import { onMounted, onUnmounted } from "vue";
import { renderMarkdown } from "../lib/markdown";
import type { FilePreviewState } from "../types";
import AppIcon from "./AppIcon.vue";

defineProps<{
  preview: FilePreviewState;
  theme: "dark" | "light";
}>();

const emit = defineEmits<{
  close: [];
}>();

function handleKeydown(event: KeyboardEvent) {
  if (event.key === "Escape") {
    emit("close");
  }
}

onMounted(() => {
  document.addEventListener("keydown", handleKeydown);
});

onUnmounted(() => {
  document.removeEventListener("keydown", handleKeydown);
});
</script>

<template>
  <Teleport to="body">
    <div class="file-preview-overlay" :data-theme="theme" @click.self="$emit('close')">
      <div
        class="file-preview-dialog"
        :class="`file-preview-dialog--${preview.kind}`"
        role="dialog"
        aria-modal="true"
        :aria-label="`${preview.filename} preview`"
      >
        <div class="file-preview-toolbar">
          <span class="file-preview-title" :title="preview.filename">{{ preview.filename }}</span>
          <div class="file-preview-actions">
            <a
              class="file-preview-btn"
              :href="preview.downloadSrc"
              :download="preview.filename"
              title="Download file"
              aria-label="Download file"
            >
              <AppIcon name="download" :size="17" />
            </a>
            <button
              type="button"
              class="file-preview-btn"
              title="Close preview"
              aria-label="Close preview"
              @click="$emit('close')"
            >
              <AppIcon name="x" :size="17" />
            </button>
          </div>
        </div>
        <div
          class="file-preview-body"
          :class="{
            'file-preview-body--image': preview.kind === 'image',
            'file-preview-body--pdf': preview.kind === 'pdf',
            'file-preview-body--text': preview.kind === 'text' || preview.kind === 'markdown',
          }"
        >
          <p v-if="preview.isLoading" class="file-preview-status">Loading preview...</p>
          <p v-else-if="preview.error" class="file-preview-status">{{ preview.error }}</p>
          <img
            v-else-if="preview.kind === 'image'"
            class="file-preview-img"
            :src="preview.src"
            :alt="preview.filename"
          />
          <iframe
            v-else-if="preview.kind === 'pdf'"
            class="file-preview-frame"
            :src="preview.src"
            :title="preview.filename"
          />
          <div
            v-else-if="preview.kind === 'markdown'"
            class="file-preview-text file-preview-markdown"
            v-html="renderMarkdown(preview.text)"
          />
          <pre v-else-if="preview.kind === 'text'" class="file-preview-text">{{
            preview.text
          }}</pre>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.file-preview-overlay {
  --bg: #0f1115;
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
  position: fixed;
  inset: 0;
  background: rgba(4, 9, 20, 0.82);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 200;
  padding: 1.5rem;
}

.file-preview-overlay[data-theme="light"] {
  --bg: #f6f7f9;
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

.file-preview-dialog {
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

.file-preview-dialog--pdf,
.file-preview-dialog--text,
.file-preview-dialog--markdown {
  width: min(90vw, 1000px);
}

.file-preview-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.5rem 0.6rem;
  border-bottom: 1px solid var(--border);
  flex: 0 0 auto;
}

.file-preview-title {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text);
  font-size: 0.84rem;
  font-weight: 650;
}

.file-preview-actions {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  flex: 0 0 auto;
}

.file-preview-btn {
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

.file-preview-btn:hover {
  color: var(--text);
  background: var(--surface-hover);
  border-color: var(--border);
}

.file-preview-body {
  overflow: auto;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
}

.file-preview-body--pdf {
  align-items: stretch;
  justify-content: stretch;
  padding: 0;
  height: min(78vh, 760px);
}

.file-preview-body--text {
  align-items: stretch;
  justify-content: stretch;
  padding: 0;
}

.file-preview-img {
  max-width: 100%;
  max-height: calc(90vh - 6rem);
  object-fit: contain;
  border-radius: 0.35rem;
  display: block;
}

.file-preview-frame {
  width: 100%;
  height: 100%;
  border: 0;
  background: #fff;
}

.file-preview-text {
  width: 100%;
  box-sizing: border-box;
  max-height: calc(90vh - 5.2rem);
  margin: 0;
  padding: 1rem;
  overflow: auto;
  background: var(--surface-soft);
  color: var(--text);
  font:
    0.84rem/1.55 ui-monospace,
    SFMono-Regular,
    Menlo,
    Consolas,
    monospace;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.file-preview-markdown {
  font-family: inherit;
  font-size: 0.94rem;
  line-height: 1.55;
  white-space: normal;
}

.file-preview-markdown :deep(> :first-child) {
  margin-top: 0;
}

.file-preview-markdown :deep(> :last-child) {
  margin-bottom: 0;
}

.file-preview-markdown :deep(pre) {
  overflow-x: auto;
  padding: 0.72rem;
  border-radius: 0.5rem;
  background: color-mix(in srgb, var(--surface) 76%, var(--surface-soft));
}

.file-preview-status {
  margin: 0;
  padding: 2rem;
  color: var(--muted);
}

@media (max-width: 640px) {
  .file-preview-overlay {
    padding: 0.5rem;
  }

  .file-preview-dialog {
    max-width: 100%;
    max-height: calc(100dvh - 1rem);
    border-radius: 0.65rem;
  }

  .file-preview-dialog--pdf,
  .file-preview-dialog--text,
  .file-preview-dialog--markdown {
    width: 100%;
  }

  .file-preview-body {
    padding: 0.6rem;
  }

  .file-preview-body--pdf,
  .file-preview-body--text {
    padding: 0;
  }

  .file-preview-body--pdf {
    height: calc(100dvh - 5rem);
  }

  .file-preview-text {
    max-height: calc(100dvh - 5rem);
    padding: 0.8rem;
  }
}
</style>
