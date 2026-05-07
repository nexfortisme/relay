<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'
import AppIcon from '../components/AppIcon.vue'
import AppSidebar from '../components/AppSidebar.vue'
import PageNavTabs from '../components/PageNavTabs.vue'
import {
  checkFeed,
  createFeed,
  deleteFeed,
  getFeedItem,
  listFeedItems,
  listFeeds,
  markFeedRead,
  summarizeFeedItem,
  updateFeed,
  updateFeedItem,
  type Feed,
  type FeedCheckResult,
  type FeedItem,
  type FeedItemView,
} from '../lib/api'
import { renderMarkdown } from '../lib/markdown'
import { useUiStore } from '../stores/uiStore'

type BackfillMode = 'latest' | 'since' | 'all'
type SummaryMode = 'summary' | 'resummary' | 'expanded'

const FEED_QUERY_KEY = 'feedId'
const POST_QUERY_KEY = 'postId'

const route = useRoute()
const router = useRouter()
const uiStore = useUiStore()
const { isSidebarCollapsed } = storeToRefs(uiStore)

const feeds = ref<Feed[]>([])
const items = ref<FeedItem[]>([])
const selectedItem = ref<FeedItem | null>(null)
const selectedItemId = ref<string | null>(null)
const selectedFeedId = ref<string | null>(null)
const activeView = ref<FeedItemView>('unread')
const starredCount = ref(0)
const feedSearch = ref('')
const feedsLoading = ref(false)
const itemsLoading = ref(false)
const manualRefreshLoading = ref(false)
const markAllReadLoading = ref(false)
const feedError = ref('')
const itemError = ref('')
const summarizingMode = ref<SummaryMode | null>(null)
const snackbarMessage = ref('')
const itemListPaneEl = ref<HTMLElement | null>(null)
const summaryTargetCharacters = ref(150)

const showFeedDialog = ref(false)
const editingFeed = ref<Feed | null>(null)
const checkingFeed = ref(false)
const savingFeed = ref(false)
const deletingFeed = ref(false)
const confirmingDeleteFeed = ref(false)
const dialogError = ref('')
const checkResult = ref<FeedCheckResult | null>(null)

const feedForm = reactive({
  url: '',
  name: '',
  pollingIntervalMinutes: 30,
  autoSummarize: true,
  autoAddToNotebook: false,
  notebookId: '',
  backfillMode: 'latest' as BackfillMode,
  backfillLimit: 20,
  backfillSince: defaultSinceDate(),
})

let readTimer: ReturnType<typeof setTimeout> | null = null
let snackbarTimer: ReturnType<typeof setTimeout> | null = null
let summaryTargetObserver: ResizeObserver | null = null
const pendingReadRemovalId = ref<string | null>(null)

const totalUnread = computed(() =>
  feeds.value.reduce((sum, feed) => sum + Math.max(feed.unreadCount, 0), 0),
)
const filteredFeeds = computed(() => {
  const query = feedSearch.value.trim().toLowerCase()
  if (!query) {
    return feeds.value
  }
  return feeds.value.filter((feed) => feed.title.toLowerCase().includes(query))
})
const activeFeed = computed(() => feeds.value.find((feed) => feed.id === selectedFeedId.value))
const selectedFeedShowsAll = computed(
  () => selectedFeedId.value !== null && activeView.value === 'all',
)
const canMarkSelectedFeedRead = computed(
  () =>
    activeView.value === 'unread' &&
    (activeFeed.value?.unreadCount ?? 0) > 0 &&
    !itemsLoading.value &&
    !markAllReadLoading.value,
)
const itemListTitle = computed(() => {
  if (activeFeed.value) {
    return activeFeed.value.title
  }
  if (activeView.value === 'starred') {
    return 'Starred'
  }
  if (activeView.value === 'all') {
    return 'All items'
  }
  return 'All unread'
})
const canSaveFeed = computed(
  () =>
    editingFeed.value !== null ||
    (feedForm.url.trim().length > 0 && feedForm.pollingIntervalMinutes >= 5),
)

onMounted(async () => {
  await refreshFeeds()
  await hydrateFromRoute()
  startSummaryTargetObserver()
})

watch(
  () => ({ feedId: route.query[FEED_QUERY_KEY], postId: route.query[POST_QUERY_KEY] }),
  async () => {
    if (routeQueryMatchesState()) {
      return
    }
    await hydrateFromRoute()
  },
)

onBeforeUnmount(() => {
  clearReadTimer()
  clearSnackbarTimer()
  stopSummaryTargetObserver()
  commitPendingReadRemoval()
})

async function refreshFeeds() {
  feedsLoading.value = true
  feedError.value = ''
  try {
    feeds.value = await listFeeds()
    await refreshStarredCount()
  } catch (error) {
    feedError.value = error instanceof Error ? error.message : 'Failed to load feeds'
  } finally {
    feedsLoading.value = false
  }
}

async function refreshStarredCount() {
  try {
    starredCount.value = (await listFeedItems('starred')).length
  } catch {
    starredCount.value = 0
  }
}

function buildQueryFromState(): Record<string, string> {
  const q: Record<string, string> = {}
  if (selectedFeedId.value) {
    q[FEED_QUERY_KEY] = selectedFeedId.value
  }
  if (selectedItemId.value) {
    q[POST_QUERY_KEY] = selectedItemId.value
  }
  return q
}

function routeQueryMatchesState(): boolean {
  const q = route.query
  const routeFeed = typeof q[FEED_QUERY_KEY] === 'string' ? q[FEED_QUERY_KEY] : ''
  const routePost = typeof q[POST_QUERY_KEY] === 'string' ? q[POST_QUERY_KEY] : ''
  const stateFeed = selectedFeedId.value ?? ''
  const statePost = selectedItemId.value ?? ''
  return routeFeed === stateFeed && routePost === statePost
}

async function syncRouterQueryFromState() {
  const next = buildQueryFromState()
  const q = route.query
  const curFeed = typeof q[FEED_QUERY_KEY] === 'string' ? q[FEED_QUERY_KEY] : ''
  const curPost = typeof q[POST_QUERY_KEY] === 'string' ? q[POST_QUERY_KEY] : ''
  const nextFeed = next[FEED_QUERY_KEY] ?? ''
  const nextPost = next[POST_QUERY_KEY] ?? ''
  if (curFeed === nextFeed && curPost === nextPost) {
    return
  }
  await router.replace({ path: route.path, query: next })
}

function applyFeedSelectionFromQuery() {
  const raw = route.query[FEED_QUERY_KEY]
  const fid = typeof raw === 'string' && raw.trim() ? raw : null
  if (fid && feeds.value.some((f) => f.id === fid)) {
    activeView.value = 'unread'
    selectedFeedId.value = fid
    return
  }
  selectedFeedId.value = null
}

async function applyPostSelectionFromQuery() {
  const raw = route.query[POST_QUERY_KEY]
  const postId = typeof raw === 'string' && raw.trim() ? raw : null
  if (!postId) {
    return
  }
  let row = items.value.find((item) => item.id === postId)
  if (row) {
    await selectItem(row)
    return
  }
  try {
    const detail = await getFeedItem(postId)
    if (selectedFeedId.value !== detail.feedId) {
      selectedFeedId.value = detail.feedId
    }
    if (activeView.value === 'unread' && detail.read) {
      activeView.value = 'all'
    }
    await refreshItems({ syncUrl: false })
    row = items.value.find((item) => item.id === postId)
    if (row) {
      await selectItem(row)
    } else {
      await selectItem(detail)
    }
  } catch (error) {
    itemError.value = error instanceof Error ? error.message : 'Failed to load feed item'
  }
}

async function hydrateFromRoute() {
  applyFeedSelectionFromQuery()
  await refreshItems({ syncUrl: false })
  await applyPostSelectionFromQuery()
  await syncRouterQueryFromState()
}

async function refreshItems(options?: { syncUrl?: boolean }): Promise<FeedItem[] | null> {
  const syncUrl = options?.syncUrl !== false
  commitPendingReadRemoval()
  clearReadTimer()
  selectedItem.value = null
  selectedItemId.value = null
  itemsLoading.value = true
  itemError.value = ''
  let result: FeedItem[] | null = null
  try {
    const refreshedItems = await listFeedItems(activeView.value, selectedFeedId.value)
    items.value = refreshedItems
    result = refreshedItems
  } catch (error) {
    itemError.value = error instanceof Error ? error.message : 'Failed to load feed items'
    result = null
  } finally {
    itemsLoading.value = false
  }
  if (syncUrl) {
    await syncRouterQueryFromState()
  }
  return result
}

async function refreshItemsWithFeedback() {
  if (manualRefreshLoading.value) {
    return
  }
  manualRefreshLoading.value = true
  try {
    const refreshedItems = await refreshItems()
    if (refreshedItems) {
      showSnackbar(refreshResultMessage(refreshedItems.length))
    }
  } finally {
    manualRefreshLoading.value = false
  }
}

async function markSelectedFeedRead() {
  const feedId = selectedFeedId.value
  if (!feedId || activeView.value !== 'unread' || markAllReadLoading.value) {
    return
  }
  markAllReadLoading.value = true
  itemError.value = ''
  clearReadTimer()
  pendingReadRemovalId.value = null
  selectedItem.value = null
  selectedItemId.value = null
  try {
    const result = await markFeedRead(feedId)
    feeds.value = feeds.value.map((feed) => (feed.id === result.feed.id ? result.feed : feed))
    await refreshFeeds()
    await refreshItems()
    showSnackbar(markReadResultMessage(result.updatedCount))
  } catch (error) {
    itemError.value = error instanceof Error ? error.message : 'Failed to mark feed read'
  } finally {
    markAllReadLoading.value = false
  }
}

async function chooseView(view: FeedItemView) {
  activeView.value = view
  selectedFeedId.value = null
  await refreshItems()
}

async function chooseFeed(feedId: string) {
  activeView.value = 'unread'
  selectedFeedId.value = selectedFeedId.value === feedId ? null : feedId
  await refreshItems()
}

async function toggleSelectedFeedAll() {
  if (!selectedFeedId.value) {
    return
  }
  activeView.value = selectedFeedShowsAll.value ? 'unread' : 'all'
  await refreshItems()
}

async function selectItem(item: FeedItem) {
  if (selectedItemId.value === item.id) {
    await syncRouterQueryFromState()
    return
  }
  commitPendingReadRemoval(item.id)
  clearReadTimer()
  selectedItemId.value = item.id
  itemError.value = ''
  if (!item.read) {
    scheduleReadTimer(item.id)
  }
  try {
    const detail = await getFeedItem(item.id)
    if (selectedItemId.value !== item.id) {
      return
    }
    const readDetail =
      pendingReadRemovalId.value === item.id ? { ...detail, read: true } : detail
    selectedItem.value = readDetail
    if (readDetail.read) {
      clearReadTimer()
    } else if (item.read) {
      scheduleReadTimer(detail.id)
    }
    await syncRouterQueryFromState()
  } catch (error) {
    if (selectedItemId.value === item.id) {
      clearReadTimer()
      itemError.value = error instanceof Error ? error.message : 'Failed to load feed item'
    }
  }
}

function scheduleReadTimer(itemId: string) {
  readTimer = setTimeout(() => {
    readTimer = null
    void markItemReadAfterDwell(itemId)
  }, 5000)
}

async function markItemReadAfterDwell(itemId: string) {
  if (selectedItemId.value !== itemId) {
    return
  }
  try {
    const updated = await updateFeedItem(itemId, { read: true })
    patchLocalItem(updated, { keepUnreadVisible: true })
    if (selectedItemId.value === itemId) {
      selectedItem.value = updated
      pendingReadRemovalId.value = itemId
    }
    await refreshFeeds()
  } catch (error) {
    console.error('failed to mark feed item read', error)
  }
}

function commitPendingReadRemoval(nextItemId?: string) {
  const itemId = pendingReadRemovalId.value
  if (!itemId || itemId === nextItemId) {
    return
  }
  if (activeView.value === 'unread') {
    items.value = items.value.filter((item) => item.id !== itemId)
  }
  pendingReadRemovalId.value = null
}

function clearReadTimer() {
  if (!readTimer) {
    return
  }
  clearTimeout(readTimer)
  readTimer = null
}

function refreshResultMessage(count: number): string {
  if (count === 0) {
    return 'Nothing was found.'
  }
  if (count === 1) {
    return '1 item found.'
  }
  return `${count} items found.`
}

function markReadResultMessage(count: number): string {
  if (count === 0) {
    return 'Nothing to mark as read.'
  }
  if (count === 1) {
    return 'Marked 1 item read.'
  }
  return `Marked ${count} items read.`
}

function showSnackbar(message: string) {
  clearSnackbarTimer()
  snackbarMessage.value = message
  snackbarTimer = setTimeout(() => {
    snackbarMessage.value = ''
    snackbarTimer = null
  }, 3200)
}

function clearSnackbarTimer() {
  if (!snackbarTimer) {
    return
  }
  clearTimeout(snackbarTimer)
  snackbarTimer = null
}

async function toggleSelectedStar() {
  if (!selectedItem.value) {
    return
  }
  const updated = await updateFeedItem(selectedItem.value.id, {
    starred: !selectedItem.value.starred,
  })
  selectedItem.value = updated
  patchLocalItem(updated)
  await refreshStarredCount()
}

async function runSummary(mode: SummaryMode) {
  if (!selectedItem.value || summarizingMode.value || isVideoItem(selectedItem.value)) {
    return
  }
  const itemId = selectedItem.value.id
  summarizingMode.value = mode
  itemError.value = ''
  const workingItem = {
    ...selectedItem.value,
    summaryStatus: 'working',
    summaryError: '',
  }
  selectedItem.value = workingItem
  patchLocalItem(workingItem, { keepUnreadVisible: true })
  try {
    const updated = await summarizeFeedItem(itemId, mode, summaryTargetCharacters.value)
    if (selectedItemId.value === itemId) {
      selectedItem.value = updated
    }
    patchLocalItem(updated, { keepUnreadVisible: true })
  } catch (error) {
    if (selectedItemId.value === itemId) {
      const errorItem = {
        ...workingItem,
        summaryStatus: 'error',
        summaryError: error instanceof Error ? error.message : 'Failed to summarize item',
      }
      selectedItem.value = errorItem
      patchLocalItem(errorItem, { keepUnreadVisible: true })
    }
    itemError.value = error instanceof Error ? error.message : 'Failed to summarize item'
  } finally {
    summarizingMode.value = null
  }
}

function patchLocalItem(updated: FeedItem, options?: { keepUnreadVisible?: boolean }) {
  const index = items.value.findIndex((item) => item.id === updated.id)
  const shouldRemoveUnread =
    activeView.value === 'unread' && updated.read && !options?.keepUnreadVisible
  const shouldRemoveStarred = activeView.value === 'starred' && !updated.starred
  if (index >= 0) {
    if (shouldRemoveUnread || shouldRemoveStarred) {
      items.value.splice(index, 1)
      return
    }
    items.value[index] = { ...items.value[index], ...updated }
  }
}

function openOriginal() {
  if (!selectedItem.value?.url) {
    return
  }
  window.open(selectedItem.value.url, '_blank', 'noopener,noreferrer')
}

function openAddDialog() {
  editingFeed.value = null
  confirmingDeleteFeed.value = false
  resetFeedForm()
  showFeedDialog.value = true
}

function openEditDialog(feed: Feed) {
  editingFeed.value = feed
  confirmingDeleteFeed.value = false
  feedForm.url = feed.url
  feedForm.name = feed.title
  feedForm.pollingIntervalMinutes = feed.pollingIntervalMinutes
  feedForm.autoSummarize = feed.autoSummarize
  feedForm.autoAddToNotebook = false
  feedForm.notebookId = feed.notebookId ?? ''
  feedForm.backfillMode = 'latest'
  feedForm.backfillLimit = 20
  feedForm.backfillSince = defaultSinceDate()
  checkResult.value = null
  dialogError.value = ''
  showFeedDialog.value = true
}

function closeFeedDialog() {
  if (deletingFeed.value) {
    return
  }
  showFeedDialog.value = false
  confirmingDeleteFeed.value = false
}

function resetFeedForm() {
  feedForm.url = ''
  feedForm.name = ''
  feedForm.pollingIntervalMinutes = 30
  feedForm.autoSummarize = true
  feedForm.autoAddToNotebook = false
  feedForm.notebookId = ''
  feedForm.backfillMode = 'latest'
  feedForm.backfillLimit = 20
  feedForm.backfillSince = defaultSinceDate()
  checkResult.value = null
  dialogError.value = ''
}

async function detectFeed() {
  checkingFeed.value = true
  dialogError.value = ''
  checkResult.value = null
  try {
    const result = await checkFeed(feedForm.url)
    checkResult.value = result
    feedForm.url = result.url
    if (!feedForm.name.trim()) {
      feedForm.name = result.title
    }
  } catch (error) {
    dialogError.value = error instanceof Error ? error.message : 'Feed check failed'
  } finally {
    checkingFeed.value = false
  }
}

async function saveFeed() {
  if (!canSaveFeed.value || savingFeed.value || deletingFeed.value) {
    return
  }
  savingFeed.value = true
  dialogError.value = ''
  try {
    if (editingFeed.value) {
      await updateFeed(editingFeed.value.id, {
        name: feedForm.name,
        pollingIntervalMinutes: feedForm.pollingIntervalMinutes,
        autoSummarize: feedForm.autoSummarize,
        autoAddToNotebook: false,
        notebookId: feedForm.notebookId,
      })
    } else {
      await createFeed({
        url: feedForm.url,
        name: feedForm.name,
        pollingIntervalMinutes: feedForm.pollingIntervalMinutes,
        autoSummarize: feedForm.autoSummarize,
        autoAddToNotebook: false,
        notebookId: feedForm.notebookId,
        backfill: {
          mode: feedForm.backfillMode,
          limit: feedForm.backfillMode === 'latest' ? feedForm.backfillLimit : undefined,
          since: feedForm.backfillMode === 'since' ? feedForm.backfillSince : undefined,
        },
      })
    }
    closeFeedDialog()
    await refreshFeeds()
    await refreshItems()
  } catch (error) {
    dialogError.value = error instanceof Error ? error.message : 'Failed to save feed'
  } finally {
    savingFeed.value = false
  }
}

async function deleteEditingFeed() {
  if (!editingFeed.value || deletingFeed.value) {
    return
  }
  const feedId = editingFeed.value.id
  deletingFeed.value = true
  dialogError.value = ''
  try {
    await deleteFeed(feedId)
    if (selectedFeedId.value === feedId) {
      selectedFeedId.value = null
    }
    if (selectedItem.value?.feedId === feedId) {
      selectedItem.value = null
      selectedItemId.value = null
    }
    showFeedDialog.value = false
    confirmingDeleteFeed.value = false
    showSnackbar('Feed deleted.')
    await refreshFeeds()
    await refreshItems()
  } catch (error) {
    dialogError.value = error instanceof Error ? error.message : 'Failed to delete feed'
  } finally {
    deletingFeed.value = false
    confirmingDeleteFeed.value = false
  }
}

function defaultSinceDate(): string {
  const date = new Date()
  date.setDate(date.getDate() - 7)
  return date.toISOString().slice(0, 10)
}

function formatItemDate(value?: string): string {
  if (!value) {
    return ''
  }
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  }).format(new Date(value))
}

function itemMeta(item: FeedItem): string {
  return [item.feedTitle, item.author, formatItemDate(item.publishedAt)].filter(Boolean).join(' · ')
}

function itemDescription(item: FeedItem): string {
  if (item.summaryStatus === 'working') {
    return 'Summarizing...'
  }
  if (item.summary && item.summaryStatus !== 'error') {
    return plainSummaryText(item.summary) || item.preview
  }
  return item.preview
}

function plainSummaryText(value: string): string {
  return value
    .replace(/^\s{0,3}#{1,6}\s+/gm, '')
    .replace(/^\s*[-*+]\s+/gm, '')
    .replace(/^\s*\d+\.\s+/gm, '')
    .replace(/[`*_]/g, '')
    .replace(/\s+/g, ' ')
    .trim()
}

function startSummaryTargetObserver() {
  updateSummaryTargetCharacters()
  if (!itemListPaneEl.value || typeof ResizeObserver === 'undefined') {
    return
  }
  summaryTargetObserver = new ResizeObserver(() => {
    updateSummaryTargetCharacters()
  })
  summaryTargetObserver.observe(itemListPaneEl.value)
}

function stopSummaryTargetObserver() {
  summaryTargetObserver?.disconnect()
  summaryTargetObserver = null
}

function updateSummaryTargetCharacters() {
  const width = itemListPaneEl.value?.getBoundingClientRect().width ?? 0
  if (width <= 0) {
    return
  }
  const usableWidth = Math.max(0, width - 28)
  summaryTargetCharacters.value = clamp(Math.round(usableWidth / 7), 80, 220)
}

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value))
}

function isVideoItem(item: FeedItem): boolean {
  return Boolean(item.mediaType && item.mediaUrl)
}

function videoPreviewLabel(item: FeedItem): string {
  switch (item.mediaType) {
    case 'youtube':
      return 'YouTube preview'
    case 'vimeo':
      return 'Vimeo preview'
    default:
      return 'Video preview'
  }
}

function videoEmbedUrl(item: FeedItem): string {
  if (!item.mediaUrl) {
    return ''
  }
  if (item.mediaType === 'youtube') {
    return youtubeEmbedUrl(item.mediaUrl)
  }
  if (item.mediaType === 'vimeo') {
    return vimeoEmbedUrl(item.mediaUrl)
  }
  return item.mediaUrl
}

function youtubeEmbedUrl(rawUrl: string): string {
  try {
    const url = new URL(rawUrl)
    const host = url.hostname.replace(/^www\./, '')
    if (host === 'youtu.be') {
      return `https://www.youtube.com/embed/${url.pathname.replace(/^\//, '')}`
    }
    if (url.pathname.startsWith('/shorts/')) {
      return `https://www.youtube.com/embed/${url.pathname.split('/')[2] ?? ''}`
    }
    if (url.pathname.startsWith('/embed/')) {
      return rawUrl
    }
    const id = url.searchParams.get('v')
    return id ? `https://www.youtube.com/embed/${id}` : rawUrl
  } catch {
    return rawUrl
  }
}

function vimeoEmbedUrl(rawUrl: string): string {
  try {
    const url = new URL(rawUrl)
    const host = url.hostname.replace(/^www\./, '')
    if (host === 'player.vimeo.com') {
      return rawUrl
    }
    const id = url.pathname.split('/').filter(Boolean)[0]
    return id ? `https://player.vimeo.com/video/${id}` : rawUrl
  } catch {
    return rawUrl
  }
}
</script>

<template>
  <div class="feeds-view" :class="{ 'feeds-view--sidebar-collapsed': isSidebarCollapsed }">
    <AppSidebar v-if="!isSidebarCollapsed" />
    <button
      v-if="!isSidebarCollapsed"
      class="mobile-sidebar-backdrop"
      type="button"
      aria-label="Close sidebar"
      @click="uiStore.toggleSidebarCollapsed"
    />

    <section class="feeds-shell">
      <PageNavTabs />
      <header class="feeds-header">
        <div>
          <h1>Feeds</h1>
          <p>{{ totalUnread }} unread</p>
        </div>
        <div class="header-actions">
          <button
            class="icon-btn refresh-action"
            type="button"
            :title="manualRefreshLoading ? 'Refreshing feeds' : 'Refresh feeds'"
            :aria-label="manualRefreshLoading ? 'Refreshing feeds' : 'Refresh feeds'"
            :aria-busy="manualRefreshLoading"
            :disabled="manualRefreshLoading"
            @click="refreshItemsWithFeedback"
          >
            <span class="refresh-icon" :class="{ 'refresh-icon--spinning': manualRefreshLoading }">
              <AppIcon name="refresh" :size="17" />
            </span>
          </button>
          <button class="primary-action" type="button" @click="openAddDialog">
            <AppIcon name="plus" :size="16" />
            Add feed
          </button>
        </div>
      </header>

      <div class="feeds-reader">
        <aside class="feed-rail">
          <div class="feed-search">
            <input v-model="feedSearch" type="search" placeholder="Filter feeds" />
          </div>

          <nav class="feed-filters" aria-label="Feed filters">
            <button
              class="filter-row"
              :class="{ 'filter-row--active': activeView === 'unread' && !selectedFeedId }"
              type="button"
              @click="chooseView('unread')"
            >
              <span><AppIcon name="inbox" :size="15" /> All unread</span>
              <strong class="filter-count">{{ totalUnread }}</strong>
            </button>
            <button
              class="filter-row"
              :class="{ 'filter-row--active': activeView === 'starred' && !selectedFeedId }"
              type="button"
              @click="chooseView('starred')"
            >
              <span><AppIcon name="star" :size="15" /> Starred</span>
              <strong class="filter-count">{{ starredCount }}</strong>
            </button>
            <button
              class="filter-row"
              :class="{ 'filter-row--active': activeView === 'all' && !selectedFeedId }"
              type="button"
              @click="chooseView('all')"
            >
              <span><AppIcon name="file" :size="15" /> All items</span>
            </button>
          </nav>

          <div class="feed-rail-title">Feeds</div>
          <p v-if="feedError" class="inline-error">{{ feedError }}</p>
          <div class="feed-list">
            <div
              v-for="feed in filteredFeeds"
              :key="feed.id"
              class="feed-row"
              :class="{ 'feed-row--active': selectedFeedId === feed.id }"
            >
              <button class="feed-main" type="button" @click="chooseFeed(feed.id)">
                <span class="feed-dot" />
                <span class="feed-row-title">{{ feed.title }}</span>
                <strong v-if="feed.unreadCount">{{ feed.unreadCount }}</strong>
              </button>
              <button
                class="feed-settings"
                type="button"
                title="Feed settings"
                @click="openEditDialog(feed)"
              >
                <AppIcon name="settings" :size="14" />
              </button>
            </div>
          </div>
          <p v-if="!feedsLoading && feeds.length === 0" class="empty-small">No feeds yet.</p>
        </aside>

        <section ref="itemListPaneEl" class="item-list-pane">
          <div class="item-list-header">
            <div>
              <h2>{{ itemListTitle }}</h2>
              <p>{{ items.length }} items</p>
            </div>
            <div v-if="activeFeed" class="item-list-actions">
              <button
                v-if="activeView === 'unread'"
                class="secondary-action mark-read-action"
                type="button"
                :disabled="!canMarkSelectedFeedRead"
                @click="markSelectedFeedRead"
              >
                <AppIcon name="check" :size="15" />
                {{ markAllReadLoading ? 'Marking' : 'Mark all read' }}
              </button>
              <label class="feed-view-toggle">
                <span>Show all</span>
                <input
                  type="checkbox"
                  :checked="selectedFeedShowsAll"
                  @change="toggleSelectedFeedAll"
                />
                <span class="toggle-track" aria-hidden="true">
                  <span class="toggle-thumb" />
                </span>
              </label>
            </div>
          </div>
          <p v-if="itemError" class="inline-error">{{ itemError }}</p>
          <div class="item-list">
            <button
              v-for="item in items"
              :key="item.id"
              class="item-row"
              :class="{
                'item-row--selected': selectedItemId === item.id,
                'item-row--read': item.read,
                'item-row--read-dimmed': activeView === 'unread' && item.read,
              }"
              type="button"
              @click="selectItem(item)"
            >
              <span class="item-title-line">
                <strong>{{ item.title }}</strong>
                <span class="item-badges">
                  <span v-if="isVideoItem(item)" class="item-badge">Video</span>
                  <AppIcon v-if="item.starred" class="item-star-icon" name="star" :size="14" filled />
                </span>
              </span>
              <span class="item-meta">{{ itemMeta(item) }}</span>
              <span
                class="item-preview"
                :class="{ 'item-preview--summarizing': item.summaryStatus === 'working' }"
              >
                {{ itemDescription(item) }}
              </span>
            </button>
          </div>
          <p v-if="!itemsLoading && items.length === 0" class="empty-state">Nothing here.</p>
        </section>

        <article class="item-panel">
          <div v-if="!selectedItem" class="panel-empty">
            <AppIcon name="inbox" :size="26" />
          </div>
          <template v-else>
            <header class="panel-header">
              <div>
                <h2>{{ selectedItem.title }}</h2>
                <p>{{ itemMeta(selectedItem) }}</p>
              </div>
              <div class="panel-actions">
                <button class="icon-btn" type="button" title="Open original" @click="openOriginal">
                  <AppIcon name="external-link" :size="16" />
                </button>
                <button
                  class="icon-btn"
                  type="button"
                  :title="isVideoItem(selectedItem) ? 'Summary unavailable for video items' : 'Summarize'"
                  :disabled="summarizingMode !== null || isVideoItem(selectedItem)"
                  @click="runSummary('summary')"
                >
                  <AppIcon name="sparkles" :size="16" />
                </button>
                <button
                  class="icon-btn"
                  type="button"
                  :title="isVideoItem(selectedItem) ? 'Summary unavailable for video items' : 'Resummarize'"
                  :disabled="summarizingMode !== null || isVideoItem(selectedItem)"
                  @click="runSummary('resummary')"
                >
                  <AppIcon name="refresh" :size="16" />
                </button>
                <button
                  class="text-action"
                  type="button"
                  :disabled="summarizingMode !== null || isVideoItem(selectedItem)"
                  :title="isVideoItem(selectedItem) ? 'Summary unavailable for video items' : 'Expand summary'"
                  @click="runSummary('expanded')"
                >
                  Expand
                </button>
                <button
                  class="icon-btn star-action"
                  :class="{ 'star-action--active': selectedItem.starred }"
                  type="button"
                  :title="selectedItem.starred ? 'Unstar item' : 'Star item'"
                  :aria-pressed="selectedItem.starred"
                  @click="toggleSelectedStar"
                >
                  <span
                    class="star-fade-icon"
                    :class="{ 'star-fade-icon--active': selectedItem.starred }"
                  >
                    <AppIcon class="star-fade-icon__outline" name="star" :size="16" />
                    <AppIcon class="star-fade-icon__fill" name="star" :size="16" filled />
                  </span>
                </button>
                <button class="icon-btn" type="button" title="Save to notebook" disabled>
                  <AppIcon name="book" :size="16" />
                </button>
              </div>
            </header>

            <section
              v-if="selectedItem.summary || selectedItem.summaryStatus || summarizingMode"
              class="summary-panel"
              aria-label="AI summary"
            >
              <div class="summary-title">
                <AppIcon name="sparkles" :size="16" />
                Summary
              </div>
              <p
                v-if="selectedItem.summaryStatus === 'working' || summarizingMode"
                class="summary-working-text"
              >
                Summarizing...
              </p>
              <p v-else-if="selectedItem.summaryError" class="inline-error">
                {{ selectedItem.summaryError }}
              </p>
              <div
                v-else-if="selectedItem.summary"
                class="summary-markdown"
                v-html="renderMarkdown(selectedItem.summary)"
              />
            </section>

            <section class="preview-panel">
              <div class="preview-title">
                {{ isVideoItem(selectedItem) ? videoPreviewLabel(selectedItem) : 'Preview' }}
              </div>
              <div
                v-if="selectedItem.mediaType === 'youtube' || selectedItem.mediaType === 'vimeo'"
                class="video-frame"
              >
                <iframe
                  :src="videoEmbedUrl(selectedItem)"
                  :title="selectedItem.title"
                  allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
                  allowfullscreen
                />
              </div>
              <video
                v-else-if="selectedItem.mediaType === 'video'"
                class="video-player"
                :src="videoEmbedUrl(selectedItem)"
                controls
              />
              <p>{{ selectedItem.content || selectedItem.preview }}</p>
            </section>
          </template>
        </article>
      </div>
    </section>

    <div
      v-if="showFeedDialog"
      class="feed-dialog-overlay"
      role="presentation"
      @keydown.esc.prevent
    >
      <section class="feed-dialog" role="dialog" aria-modal="true" aria-labelledby="feed-dialog-title">
        <header class="dialog-header">
          <h2 id="feed-dialog-title">{{ editingFeed ? 'Feed settings' : 'Add a feed' }}</h2>
          <span>{{ editingFeed ? 'settings' : 'dialog' }}</span>
        </header>

        <div class="dialog-body">
          <label class="field-label" for="feed-url">RSS feed URL</label>
          <div class="detect-row">
            <input
              id="feed-url"
              v-model="feedForm.url"
              type="url"
              placeholder="https://example.com/rss.xml"
              :disabled="editingFeed !== null"
            />
            <button
              type="button"
              class="secondary-action"
              :disabled="editingFeed !== null || checkingFeed || !feedForm.url.trim()"
              @click="detectFeed"
            >
              {{ checkingFeed ? 'Checking' : 'Check feed' }}
            </button>
          </div>

          <div v-if="checkResult" class="detected-feed">
            <span class="feed-dot" />
            <div>
              <strong>{{ checkResult.title || 'Untitled feed' }}</strong>
              <p>{{ checkResult.itemCount }} items detected</p>
            </div>
          </div>

          <label class="field-label" for="feed-name">Name</label>
          <input id="feed-name" v-model="feedForm.name" type="text" placeholder="Generated if blank" />

          <div class="settings-grid">
            <label>
              <span>Check every</span>
              <select v-model.number="feedForm.pollingIntervalMinutes">
                <option :value="5">5 min</option>
                <option :value="15">15 min</option>
                <option :value="30">30 min</option>
                <option :value="60">1 hour</option>
                <option :value="360">6 hours</option>
                <option :value="1440">1 day</option>
              </select>
            </label>
            <label class="check-label">
              <input v-model="feedForm.autoSummarize" type="checkbox" />
              <span>Auto-summary</span>
            </label>
            <label class="check-label">
              <input v-model="feedForm.autoAddToNotebook" type="checkbox" disabled />
              <span>Auto notebook</span>
            </label>
            <input
              v-model="feedForm.notebookId"
              type="text"
              placeholder="Notebook suggestions later"
              disabled
            />
          </div>

          <div v-if="!editingFeed" class="backfill-box">
            <label class="field-label">Backfill inbox</label>
            <div class="segmented">
              <button
                type="button"
                :class="{ active: feedForm.backfillMode === 'latest' }"
                @click="feedForm.backfillMode = 'latest'"
              >
                Latest
              </button>
              <button
                type="button"
                :class="{ active: feedForm.backfillMode === 'since' }"
                @click="feedForm.backfillMode = 'since'"
              >
                Since
              </button>
              <button
                type="button"
                :class="{ active: feedForm.backfillMode === 'all' }"
                @click="feedForm.backfillMode = 'all'"
              >
                All
              </button>
            </div>
            <input
              v-if="feedForm.backfillMode === 'latest'"
              v-model.number="feedForm.backfillLimit"
              type="number"
              min="1"
              max="500"
            />
            <input v-if="feedForm.backfillMode === 'since'" v-model="feedForm.backfillSince" type="date" />
          </div>

          <div v-if="editingFeed && confirmingDeleteFeed" class="delete-confirmation" role="alert">
            <div>
              <strong>Delete {{ editingFeed.title }}?</strong>
              <p>All saved items from this feed will be removed.</p>
            </div>
            <div class="delete-confirmation-actions">
              <button
                type="button"
                class="secondary-action"
                :disabled="deletingFeed"
                @click="confirmingDeleteFeed = false"
              >
                Keep feed
              </button>
              <button
                type="button"
                class="danger-action"
                :disabled="deletingFeed"
                @click="deleteEditingFeed"
              >
                <AppIcon name="trash" :size="15" />
                {{ deletingFeed ? 'Deleting' : 'Delete feed' }}
              </button>
            </div>
          </div>

          <p v-if="dialogError" class="inline-error">{{ dialogError }}</p>
        </div>

        <footer class="dialog-footer">
          <button
            v-if="editingFeed && !confirmingDeleteFeed"
            type="button"
            class="danger-action dialog-delete-action"
            :disabled="savingFeed || deletingFeed"
            @click="confirmingDeleteFeed = true"
          >
            <AppIcon name="trash" :size="15" />
            Delete
          </button>
          <button type="button" class="secondary-action" :disabled="deletingFeed" @click="closeFeedDialog">
            Cancel
          </button>
          <button
            type="button"
            class="primary-action"
            :disabled="!canSaveFeed || savingFeed || deletingFeed"
            @click="saveFeed"
          >
            {{ savingFeed ? 'Saving' : editingFeed ? 'Save settings' : 'Add feed' }}
          </button>
        </footer>
      </section>
    </div>
    <div v-if="snackbarMessage" class="feed-snackbar" role="status" aria-live="polite">
      {{ snackbarMessage }}
    </div>
  </div>
</template>

<style scoped>
.feeds-view {
  display: grid;
  grid-template-columns: 280px 1fr;
  height: 100%;
  overflow: hidden;
  background: var(--bg);
  position: relative;
}

.feeds-view--sidebar-collapsed {
  grid-template-columns: 1fr;
}

.feeds-shell {
  min-width: 0;
  display: grid;
  grid-template-rows: auto auto 1fr;
  overflow: hidden;
}

.feeds-header {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.85rem 1rem;
  border-bottom: 1px solid var(--border);
}

.feeds-header h1,
.item-list-header h2,
.panel-header h2 {
  margin: 0;
  font-size: 1rem;
  line-height: 1.2;
}

.feeds-header p,
.item-list-header p,
.panel-header p,
.detected-feed p {
  margin: 0.2rem 0 0;
  color: var(--muted);
  font-size: 0.78rem;
}

.header-actions,
.panel-actions,
.detect-row {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 0;
}

.primary-action,
.secondary-action,
.danger-action,
.text-action,
.icon-btn {
  border: 1px solid var(--border);
  border-radius: 0.45rem;
  min-height: 2.2rem;
  color: var(--text);
  background: var(--surface);
  cursor: pointer;
  font-weight: 650;
}

.primary-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  padding: 0 0.8rem;
  color: #fff;
  background: var(--primary);
  border-color: color-mix(in srgb, var(--primary) 45%, transparent);
}

.primary-action:hover {
  background: var(--primary-strong);
}

.secondary-action,
.danger-action,
.text-action {
  padding: 0 0.7rem;
}

.danger-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  color: var(--danger);
  border-color: color-mix(in srgb, var(--danger) 42%, var(--border));
  background: color-mix(in srgb, var(--danger) 8%, var(--surface));
}

.danger-action:hover {
  border-color: color-mix(in srgb, var(--danger) 62%, var(--border));
  background: color-mix(in srgb, var(--danger) 14%, var(--surface));
}

.icon-btn {
  width: 2.2rem;
  padding: 0;
  display: inline-grid;
  place-items: center;
  flex: 0 0 auto;
  transition:
    color 0.18s ease,
    background 0.18s ease,
    border-color 0.18s ease;
}

.star-action--active {
  color: #d99a00;
  border-color: color-mix(in srgb, #d99a00 55%, var(--border));
  background: color-mix(in srgb, #d99a00 13%, var(--surface));
}

.star-action--active:hover {
  background: color-mix(in srgb, #d99a00 18%, var(--surface-hover));
}

.star-fade-icon {
  position: relative;
  width: 1rem;
  height: 1rem;
  display: inline-grid;
  place-items: center;
}

.star-fade-icon__outline,
.star-fade-icon__fill {
  grid-area: 1 / 1;
  transition:
    opacity 0.22s ease,
    transform 0.22s ease;
}

.star-fade-icon__outline {
  opacity: 1;
}

.star-fade-icon__fill {
  color: #d99a00;
  opacity: 0;
  transform: scale(0.88);
}

.star-fade-icon--active .star-fade-icon__outline {
  opacity: 0;
}

.star-fade-icon--active .star-fade-icon__fill {
  opacity: 1;
  transform: scale(1);
}

.refresh-action {
  position: relative;
}

.refresh-icon {
  display: inline-grid;
  place-items: center;
}

.refresh-icon--spinning {
  animation: feed-refresh-spin 0.8s linear infinite;
}

@keyframes feed-refresh-spin {
  to {
    transform: rotate(360deg);
  }
}

.primary-action:disabled,
.secondary-action:disabled,
.danger-action:disabled,
.text-action:disabled,
.icon-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.feeds-reader {
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(15rem, 18rem) minmax(18rem, 24rem) minmax(0, 1fr);
  overflow: hidden;
}

.feed-rail,
.item-list-pane,
.item-panel {
  min-width: 0;
  min-height: 0;
  border-right: 1px solid var(--border);
  overflow: hidden;
}

.feed-rail {
  display: grid;
  grid-template-rows: auto auto auto 1fr auto;
  gap: 0.65rem;
  padding: 0.75rem;
}

.feed-search input,
.dialog-body input,
.dialog-body select {
  width: 100%;
  min-width: 0;
  min-height: 2.25rem;
  box-sizing: border-box;
  border-radius: 0.45rem;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  padding: 0 0.7rem;
}

.feed-filters,
.feed-list,
.item-list {
  display: grid;
  gap: 0.35rem;
  min-width: 0;
}

.feed-list,
.item-list,
.item-panel {
  overflow: auto;
}

.feed-list {
  align-content: start;
  grid-auto-rows: max-content;
}

.filter-row,
.feed-main,
.item-row {
  width: 100%;
  min-width: 0;
  border: 1px solid transparent;
  border-radius: 0.45rem;
  background: transparent;
  color: var(--text);
  cursor: pointer;
  text-align: left;
}

.filter-row,
.feed-main {
  min-height: 2.15rem;
  padding: 0.45rem 0.55rem;
  display: grid;
  align-items: center;
  gap: 0.4rem;
}

.filter-row {
  grid-template-columns: minmax(0, 1fr) auto;
}

.feed-main {
  grid-template-columns: auto minmax(0, 1fr) auto;
}

.feed-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: stretch;
  border: 1px solid transparent;
  border-radius: 0.45rem;
  align-self: start;
}

.filter-row span,
.feed-row-title {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.filter-count,
.feed-main strong {
  min-width: 1.35rem;
  min-height: 1.35rem;
  border-radius: 999px;
  display: inline-grid;
  place-items: center;
  color: #fff;
  background: var(--primary);
  font-size: 0.72rem;
}

.filter-count {
  padding: 0 0.55rem;
}

.filter-row:hover,
.feed-row:hover .feed-main,
.item-row:hover {
  background: var(--surface-hover);
}

.filter-row--active,
.feed-row--active .feed-main,
.item-row--selected {
  background: var(--selected);
}

.filter-row--active,
.feed-row--active,
.item-row--selected {
  border-color: color-mix(in srgb, var(--primary) 42%, var(--border));
}

.feed-dot {
  width: 0.68rem;
  height: 0.68rem;
  border-radius: 999px;
  border: 1px solid var(--muted);
  flex: 0 0 auto;
}

.feed-settings {
  width: 2.15rem;
  height: auto;
  border: 0;
  border-radius: 0.35rem;
  display: inline-grid;
  place-items: center;
  align-self: stretch;
  justify-self: center;
  color: var(--muted);
  background: transparent;
  cursor: pointer;
}

.feed-settings:hover {
  color: var(--text);
  background: var(--surface);
}

.feed-rail-title,
.preview-title,
.summary-title,
.field-label {
  color: var(--muted);
  font-size: 0.76rem;
  font-weight: 750;
  text-transform: uppercase;
  letter-spacing: 0;
}

.item-list-pane {
  display: grid;
  grid-template-rows: auto auto 1fr auto;
}

.item-list-header {
  padding: 0.85rem 0.9rem;
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.item-list-header > div {
  min-width: 0;
}

.item-list-actions {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.55rem;
  flex-wrap: wrap;
}

.mark-read-action {
  min-height: 2rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.35rem;
  white-space: nowrap;
}

.feed-view-toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  flex: 0 0 auto;
  color: var(--muted);
  font-size: 0.76rem;
  font-weight: 750;
  cursor: pointer;
}

.feed-view-toggle input {
  position: absolute;
  inline-size: 1px;
  block-size: 1px;
  margin: 0;
  overflow: hidden;
  clip-path: inset(50%);
  opacity: 0;
  white-space: nowrap;
}

.toggle-track {
  width: 2rem;
  height: 1.1rem;
  border-radius: 999px;
  border: 1px solid var(--border);
  display: inline-flex;
  align-items: center;
  padding: 0.12rem;
  background: var(--surface);
  transition:
    background 0.15s ease,
    border-color 0.15s ease;
}

.toggle-thumb {
  width: 0.75rem;
  height: 0.75rem;
  border-radius: 999px;
  background: var(--muted);
  transition:
    transform 0.15s ease,
    background 0.15s ease;
}

.feed-view-toggle input:checked + .toggle-track {
  border-color: color-mix(in srgb, var(--primary) 45%, var(--border));
  background: color-mix(in srgb, var(--primary) 18%, var(--surface));
}

.feed-view-toggle input:checked + .toggle-track .toggle-thumb {
  transform: translateX(0.9rem);
  background: var(--primary);
}

.feed-view-toggle input:focus-visible + .toggle-track {
  outline: 2px solid color-mix(in srgb, var(--primary) 40%, transparent);
  outline-offset: 2px;
}

.item-list {
  align-content: start;
  gap: 0;
}

.item-row {
  display: grid;
  gap: 0.3rem;
  padding: 0.75rem 0.85rem;
  border-radius: 0;
  border-width: 0 0 1px;
  border-color: var(--border);
  transition:
    background 0.18s ease,
    border-color 0.18s ease,
    opacity 0.18s ease;
}

.item-row--read {
  color: var(--muted);
}

.item-row--read-dimmed {
  opacity: 0.52;
}

.item-title-line {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.4rem;
}

.item-badges {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  flex: 0 0 auto;
}

.item-star-icon {
  color: #d99a00;
}

.item-badge {
  min-height: 1.25rem;
  border-radius: 999px;
  display: inline-grid;
  place-items: center;
  padding: 0 0.45rem;
  color: var(--muted);
  background: var(--surface);
  border: 1px solid var(--border);
  font-size: 0.68rem;
  font-weight: 700;
}

.item-title-line strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.92rem;
}

.item-meta,
.item-preview {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--muted);
  font-size: 0.78rem;
}

.item-preview--summarizing,
.summary-working-text {
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
  animation: feed-summary-shimmer 1s linear infinite;
}

@keyframes feed-summary-shimmer {
  0% {
    background-position: 200% 0;
  }
  100% {
    background-position: -20% 0;
  }
}

.item-panel {
  border-right: 0;
  padding: 0.95rem;
  display: grid;
  align-content: start;
  gap: 0.8rem;
}

.panel-empty {
  height: 100%;
  min-height: 18rem;
  display: grid;
  place-items: center;
  color: color-mix(in srgb, var(--muted) 65%, transparent);
}

.panel-header {
  min-width: 0;
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: start;
  gap: 0.8rem;
  padding-bottom: 0.75rem;
  border-bottom: 1px solid var(--border);
}

.panel-actions {
  flex-wrap: wrap;
  justify-content: flex-end;
}

.summary-panel,
.preview-panel,
.detected-feed,
.backfill-box {
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  background: var(--surface);
}

.summary-panel,
.preview-panel {
  padding: 0.9rem;
}

.summary-title {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  margin-bottom: 0.45rem;
}

.summary-markdown :deep(p),
.summary-markdown :deep(ul),
.preview-panel p {
  margin: 0.35rem 0 0;
  line-height: 1.55;
}

.preview-panel p {
  white-space: pre-wrap;
}

.video-frame,
.video-player {
  width: 100%;
  margin-top: 0.65rem;
  border-radius: 0.45rem;
  background: #000;
  overflow: hidden;
}

.video-frame {
  aspect-ratio: 16 / 9;
}

.video-frame iframe {
  width: 100%;
  height: 100%;
  border: 0;
  display: block;
}

.video-player {
  display: block;
  max-height: 24rem;
}

.inline-error {
  color: #ef767a;
  font-size: 0.82rem;
}

.empty-small,
.empty-state {
  color: var(--muted);
  font-size: 0.86rem;
}

.empty-state {
  padding: 1rem;
}

.mobile-sidebar-backdrop {
  display: none;
}

.feed-snackbar {
  position: fixed;
  left: 50%;
  bottom: 1rem;
  z-index: 90;
  max-width: min(24rem, calc(100vw - 2rem));
  padding: 0.7rem 0.9rem;
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  color: var(--text);
  background: var(--surface);
  box-shadow: var(--shadow);
  font-size: 0.86rem;
  font-weight: 650;
  text-align: center;
  transform: translateX(-50%);
}

.feed-dialog-overlay {
  position: fixed;
  inset: 0;
  z-index: 80;
  display: grid;
  place-items: center;
  padding: 1rem;
  background: rgba(4, 9, 20, 0.58);
}

.feed-dialog {
  width: min(46rem, 100%);
  max-height: min(42rem, 92vh);
  display: grid;
  grid-template-rows: auto 1fr auto;
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  background: var(--bg);
  box-shadow: var(--shadow);
  overflow: hidden;
}

.dialog-header,
.dialog-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.85rem 1rem;
  border-bottom: 1px solid var(--border);
}

.dialog-header h2 {
  margin: 0;
  font-size: 1.05rem;
}

.dialog-header span {
  color: var(--muted);
  font-size: 0.78rem;
}

.dialog-body {
  display: grid;
  gap: 0.75rem;
  padding: 1rem;
  overflow: auto;
}

.dialog-footer {
  border-top: 1px solid var(--border);
  border-bottom: 0;
  justify-content: flex-end;
}

.dialog-delete-action {
  margin-right: auto;
}

.detect-row {
  align-items: stretch;
}

.detect-row input {
  flex: 1 1 auto;
}

.detected-feed {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  padding: 0.65rem 0.75rem;
}

.settings-grid {
  display: grid;
  grid-template-columns: minmax(8rem, 1fr) auto auto minmax(10rem, 1fr);
  gap: 0.65rem;
  align-items: end;
}

.settings-grid label {
  display: grid;
  gap: 0.3rem;
  color: var(--muted);
  font-size: 0.78rem;
}

.check-label {
  min-height: 2.25rem;
  grid-template-columns: auto auto;
  align-items: center;
  justify-content: start;
  color: var(--text) !important;
}

.backfill-box {
  display: grid;
  gap: 0.55rem;
  padding: 0.75rem;
}

.delete-confirmation {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 0.75rem;
  align-items: center;
  padding: 0.75rem;
  border: 1px solid color-mix(in srgb, var(--danger) 38%, var(--border));
  border-radius: 0.5rem;
  background: color-mix(in srgb, var(--danger) 7%, var(--surface));
}

.delete-confirmation strong,
.delete-confirmation p {
  margin: 0;
}

.delete-confirmation strong {
  display: block;
  overflow-wrap: anywhere;
}

.delete-confirmation p {
  margin-top: 0.2rem;
  color: var(--muted);
  font-size: 0.82rem;
}

.delete-confirmation-actions {
  display: inline-flex;
  gap: 0.45rem;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.segmented {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 0.35rem;
}

.segmented button {
  min-height: 2rem;
  border-radius: 0.4rem;
  border: 1px solid var(--border);
  background: var(--bg);
  color: var(--muted);
  cursor: pointer;
}

.segmented button.active {
  background: var(--selected);
  color: var(--text);
  border-color: color-mix(in srgb, var(--primary) 42%, var(--border));
}

@media (max-width: 1020px) {
  .feeds-reader {
    grid-template-columns: minmax(13rem, 16rem) minmax(16rem, 22rem) minmax(20rem, 1fr);
  }

  .panel-header {
    grid-template-columns: 1fr;
  }

  .panel-actions {
    justify-content: flex-start;
  }
}

@media (max-width: 760px) {
  .feeds-view {
    grid-template-columns: 1fr;
  }

  .feeds-view:not(.feeds-view--sidebar-collapsed) :deep(.home-sidebar) {
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

  .feeds-reader {
    grid-template-columns: 1fr;
    grid-template-rows: auto minmax(14rem, 34vh) minmax(22rem, 1fr);
    overflow: auto;
  }

  .feed-rail,
  .item-list-pane,
  .item-panel {
    border-right: 0;
    border-bottom: 1px solid var(--border);
    overflow: visible;
  }

  .item-list {
    max-height: 34vh;
    overflow: auto;
  }

  .settings-grid {
    grid-template-columns: 1fr;
  }

  .delete-confirmation {
    grid-template-columns: 1fr;
  }

  .detect-row {
    display: grid;
  }
}
</style>
