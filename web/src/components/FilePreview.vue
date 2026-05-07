<script setup lang="ts">
import { computed, onMounted, onUnmounted } from "vue";
import { columnLetter, parseCsvRows } from "../lib/csvParse";
import { renderMarkdown } from "../lib/markdown";
import type { FilePreviewState } from "../types";
import AppIcon from "./AppIcon.vue";

const props = defineProps<{
  preview: FilePreviewState;
  theme: "dark" | "light";
}>();

const CSV_PREVIEW_MAX_ROWS = 5000;

const csvGrid = computed(() => {
  if (props.preview.kind !== "csv" || props.preview.isLoading || props.preview.error) {
    return null;
  }
  const { rows, truncated } = parseCsvRows(props.preview.text, CSV_PREVIEW_MAX_ROWS);
  if (rows.length === 0) {
    return { colLabels: [] as string[], header: [] as string[], body: [] as string[][], truncated };
  }
  const header = rows[0] ?? [];
  const body = rows.slice(1);
  const maxCols = Math.max(
    1,
    header.length,
    ...body.map((r) => r.length),
  );
  const pad = (cells: string[]) => {
    const next = cells.slice(0, maxCols);
    while (next.length < maxCols) {
      next.push("");
    }
    return next;
  };
  const colLabels = Array.from({ length: maxCols }, (_, i) => columnLetter(i));
  return {
    colLabels,
    header: pad(header),
    body: body.map(pad),
    truncated,
  };
});

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
            'file-preview-body--text':
              preview.kind === 'text' || preview.kind === 'markdown',
            'file-preview-body--csv': preview.kind === 'csv',
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
          <div v-else-if="preview.kind === 'csv' && csvGrid" class="file-preview-csv-shell">
            <p v-if="!csvGrid.header.length && !csvGrid.body.length" class="file-preview-status">
              This CSV has no rows to display.
            </p>
            <template v-else>
              <p v-if="csvGrid.truncated" class="file-preview-csv-truncation">
                Showing the first {{ CSV_PREVIEW_MAX_ROWS.toLocaleString() }} rows. Download the
                file to see the full data.
              </p>
              <div class="file-preview-csv-scroll">
                <table class="file-preview-csv-table" role="grid">
                  <thead>
                    <tr class="file-preview-csv-letters">
                      <th class="file-preview-csv-corner" scope="col" />
                      <th
                        v-for="(label, ci) in csvGrid.colLabels"
                        :key="`col-${ci}`"
                        class="file-preview-csv-col-label"
                        scope="col"
                      >
                        {{ label }}
                      </th>
                    </tr>
                    <tr class="file-preview-csv-sheet-header">
                      <th class="file-preview-csv-row-label" scope="row">1</th>
                      <th
                        v-for="(cell, ci) in csvGrid.header"
                        :key="`h-${ci}`"
                        class="file-preview-csv-cell file-preview-csv-cell--header"
                        scope="col"
                      >
                        {{ cell }}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(row, ri) in csvGrid.body" :key="`r-${ri}`">
                      <th class="file-preview-csv-row-label" scope="row">{{ ri + 2 }}</th>
                      <td
                        v-for="(cell, ci) in row"
                        :key="`d-${ri}-${ci}`"
                        class="file-preview-csv-cell"
                      >
                        {{ cell }}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </template>
          </div>
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

.file-preview-dialog--csv {
  width: min(96vw, 1280px);
  max-width: min(96vw, 1280px);
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

.file-preview-body--text,
.file-preview-body--csv {
  align-items: stretch;
  justify-content: stretch;
  padding: 0;
}

.file-preview-csv-shell {
  width: 100%;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.file-preview-csv-truncation {
  margin: 0;
  padding: 0.5rem 0.75rem;
  font-size: 0.78rem;
  color: var(--muted);
  border-bottom: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface-soft) 88%, var(--surface));
}

.file-preview-csv-scroll {
  --csv-letters-height: 1.75rem;
  overflow: auto;
  max-height: calc(90vh - 5.25rem);
  background: var(--surface-soft);
}

.file-preview-csv-table {
  border-collapse: collapse;
  table-layout: fixed;
  min-width: 100%;
  font-size: 0.8rem;
  line-height: 1.35;
  font-variant-numeric: tabular-nums;
  font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial,
    sans-serif;
}

.file-preview-csv-table th,
.file-preview-csv-table td {
  box-sizing: border-box;
}

.file-preview-csv-corner {
  position: sticky;
  left: 0;
  z-index: 5;
  width: 2.35rem;
  min-width: 2.35rem;
  padding: 0;
  border: 1px solid var(--border);
  border-top: none;
  border-left: none;
  background: color-mix(in srgb, var(--surface-soft) 70%, var(--border));
}

.file-preview-csv-letters .file-preview-csv-corner {
  top: 0;
}

.file-preview-csv-col-label {
  position: sticky;
  top: 0;
  z-index: 3;
  min-width: 6.5rem;
  padding: 0.28rem 0.42rem;
  text-align: center;
  font-weight: 650;
  font-size: 0.72rem;
  color: var(--muted);
  border: 1px solid var(--border);
  border-top: none;
  background: color-mix(in srgb, var(--surface-soft) 55%, var(--selected));
}

.file-preview-csv-row-label {
  position: sticky;
  left: 0;
  z-index: 4;
  width: 2.35rem;
  min-width: 2.35rem;
  padding: 0.32rem 0.28rem;
  text-align: center;
  font-weight: 600;
  font-size: 0.72rem;
  color: var(--muted);
  border: 1px solid var(--border);
  border-left: none;
  background: color-mix(in srgb, var(--surface-soft) 70%, var(--border));
}

.file-preview-csv-sheet-header .file-preview-csv-row-label {
  top: var(--csv-letters-height);
}

.file-preview-csv-sheet-header .file-preview-csv-cell--header {
  position: sticky;
  top: var(--csv-letters-height);
  z-index: 2;
}

.file-preview-csv-cell {
  min-width: 6.5rem;
  max-width: 22rem;
  padding: 0.32rem 0.48rem;
  border: 1px solid var(--border);
  color: var(--text);
  background: var(--surface);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: top;
}

.file-preview-csv-cell--header {
  font-weight: 650;
  background: color-mix(in srgb, var(--surface) 76%, var(--selected));
}

.file-preview-csv-table tbody tr:nth-child(even) .file-preview-csv-cell {
  background: color-mix(in srgb, var(--surface) 94%, var(--surface-soft));
}

.file-preview-overlay[data-theme="light"] .file-preview-csv-corner,
.file-preview-overlay[data-theme="light"] .file-preview-csv-row-label {
  background: #e9ecef;
}

.file-preview-overlay[data-theme="light"] .file-preview-csv-col-label {
  background: #dee6ef;
}

.file-preview-overlay[data-theme="light"] .file-preview-csv-cell {
  background: #ffffff;
}

.file-preview-overlay[data-theme="light"] .file-preview-csv-cell--header {
  background: #dae8f5;
}

.file-preview-overlay[data-theme="light"] .file-preview-csv-table tbody tr:nth-child(even) .file-preview-csv-cell {
  background: #f7f9fb;
}

.file-preview-overlay[data-theme="dark"] .file-preview-csv-corner,
.file-preview-overlay[data-theme="dark"] .file-preview-csv-row-label {
  background: #252d3d;
}

.file-preview-overlay[data-theme="dark"] .file-preview-csv-col-label {
  background: #2c3548;
}

.file-preview-overlay[data-theme="dark"] .file-preview-csv-cell {
  background: #1b2130;
}

.file-preview-overlay[data-theme="dark"] .file-preview-csv-cell--header {
  background: #243049;
}

.file-preview-overlay[data-theme="dark"] .file-preview-csv-table tbody tr:nth-child(even) .file-preview-csv-cell {
  background: #151a26;
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
  .file-preview-dialog--markdown,
  .file-preview-dialog--csv {
    width: 100%;
  }

  .file-preview-body {
    padding: 0.6rem;
  }

  .file-preview-body--pdf,
  .file-preview-body--text,
  .file-preview-body--csv {
    padding: 0;
  }

  .file-preview-csv-scroll {
    max-height: calc(100dvh - 5.85rem);
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
