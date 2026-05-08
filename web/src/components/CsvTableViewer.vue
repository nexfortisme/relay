<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { getCSVTableData, type CSVTableData } from '../lib/notebooks'

const props = defineProps<{
  notebookId: string
  fileId: string
  fileName: string
}>()

const data = ref<CSVTableData | null>(null)
const isLoading = ref(false)
const error = ref<string | null>(null)

onMounted(async () => {
  isLoading.value = true
  error.value = null
  try {
    data.value = await getCSVTableData(props.notebookId, props.fileId)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    isLoading.value = false
  }
})
</script>

<template>
  <div class="csv-viewer">
    <div class="csv-viewer-header">
      <span class="csv-viewer-name">{{ fileName }}</span>
      <span v-if="data" class="csv-viewer-meta">
        {{ data.columns.length }} columns &middot; {{ data.rows.length }} rows
      </span>
    </div>

    <div v-if="isLoading" class="csv-viewer-state">Loading&hellip;</div>
    <div v-else-if="error" class="csv-viewer-state csv-viewer-error">{{ error }}</div>
    <div v-else-if="data && data.columns.length === 0" class="csv-viewer-state">Empty table</div>

    <div v-else-if="data" class="csv-table-wrap">
      <table class="csv-table">
        <thead>
          <tr>
            <th v-for="col in data.columns" :key="col" class="csv-th">{{ col }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(row, ri) in data.rows" :key="ri">
            <td v-for="(cell, ci) in row" :key="ci" class="csv-td">{{ cell }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.csv-viewer {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 0.55rem;
}

.csv-viewer-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.6rem 0.9rem;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.csv-viewer-name {
  font-weight: 650;
  font-size: 0.875rem;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.csv-viewer-meta {
  font-size: 0.78rem;
  color: var(--muted);
  white-space: nowrap;
}

.csv-viewer-state {
  padding: 2rem;
  text-align: center;
  color: var(--muted);
  font-size: 0.875rem;
}

.csv-viewer-error {
  color: var(--danger);
}

.csv-table-wrap {
  overflow: auto;
  flex: 1;
}

.csv-table {
  border-collapse: collapse;
  width: max-content;
  min-width: 100%;
  font-size: 0.82rem;
}

.csv-th {
  position: sticky;
  top: 0;
  background: var(--surface-soft);
  border-bottom: 1px solid var(--border);
  border-right: 1px solid var(--border);
  padding: 0.45rem 0.75rem;
  text-align: left;
  font-weight: 700;
  white-space: nowrap;
  color: var(--muted);
  text-transform: uppercase;
  font-size: 0.72rem;
}

.csv-th:last-child {
  border-right: 0;
}

.csv-td {
  border-bottom: 1px solid var(--border);
  border-right: 1px solid var(--border);
  padding: 0.42rem 0.75rem;
  white-space: nowrap;
  max-width: 24ch;
  overflow: hidden;
  text-overflow: ellipsis;
}

.csv-td:last-child {
  border-right: 0;
}

tr:last-child .csv-td {
  border-bottom: 0;
}

tr:hover .csv-td {
  background: var(--surface-hover);
}
</style>
