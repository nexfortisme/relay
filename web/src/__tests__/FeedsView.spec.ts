import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import FeedsView from '../views/FeedsView.vue'
import {
  checkFeed,
  deleteFeed,
  getFeedItem,
  listFeedItems,
  listFeeds,
  markFeedRead,
  summarizeFeedItem,
  updateFeedItem,
  type Feed,
  type FeedItem,
} from '../lib/api'

vi.mock('../lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/api')>()
  return {
    ...actual,
    checkFeed: vi.fn<typeof actual.checkFeed>(),
    createFeed: vi.fn<typeof actual.createFeed>(),
    deleteFeed: vi.fn<typeof actual.deleteFeed>(),
    getFeedItem: vi.fn<typeof actual.getFeedItem>(),
    listFeedItems: vi.fn<typeof actual.listFeedItems>(),
    listFeeds: vi.fn<typeof actual.listFeeds>(),
    markFeedRead: vi.fn<typeof actual.markFeedRead>(),
    summarizeFeedItem: vi.fn<typeof actual.summarizeFeedItem>(),
    updateFeed: vi.fn<typeof actual.updateFeed>(),
    updateFeedItem: vi.fn<typeof actual.updateFeedItem>(),
  }
})

const feed: Feed = {
  id: 'feed-1',
  url: 'https://example.com/rss.xml',
  title: 'Example Feed',
  siteUrl: 'https://example.com',
  pollingIntervalMinutes: 30,
  autoSummarize: true,
  autoAddToNotebook: false,
  nextCheckAt: new Date().toISOString(),
  unreadCount: 1,
  createdAt: new Date().toISOString(),
  updatedAt: new Date().toISOString(),
}

const item: FeedItem = {
  id: 'item-1',
  feedId: 'feed-1',
  feedTitle: 'Example Feed',
  externalId: 'guid-1',
  title: 'First post',
  url: 'https://example.com/first',
  preview: 'A short preview',
  content: 'Full content',
  read: false,
  starred: false,
  createdAt: new Date().toISOString(),
  updatedAt: new Date().toISOString(),
}

const secondItem: FeedItem = {
  ...item,
  id: 'item-2',
  externalId: 'guid-2',
  title: 'Second post',
  url: 'https://example.com/second',
}

function mockApi() {
  vi.mocked(listFeeds).mockResolvedValue([feed])
  vi.mocked(listFeedItems).mockImplementation(async (view) => {
    if (view === 'starred') {
      return []
    }
    return [item]
  })
  vi.mocked(getFeedItem).mockResolvedValue(item)
  vi.mocked(updateFeedItem).mockResolvedValue({ ...item, read: true })
  vi.mocked(markFeedRead).mockResolvedValue({ feed: { ...feed, unreadCount: 0 }, updatedCount: 1 })
  vi.mocked(deleteFeed).mockResolvedValue(undefined)
  vi.mocked(checkFeed).mockResolvedValue({
    url: feed.url,
    title: feed.title,
    siteUrl: feed.siteUrl,
    description: '',
    itemCount: 1,
  })
}

async function mountFeeds(options?: { query?: Record<string, string> }) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/feeds', name: 'feeds', component: { template: '<div />' } }],
  })
  await router.push({ path: '/feeds', query: options?.query ?? {} })
  await router.isReady()
  const wrapper = mount(FeedsView, {
    global: {
      plugins: [createPinia(), router],
      stubs: {
        AppSidebar: true,
        PageNavTabs: true,
      },
    },
  })
  await flushPromises()
  return wrapper
}

describe('FeedsView', () => {
  beforeEach(() => {
    mockApi()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.clearAllMocks()
  })

  it('prompts the user to pick a post when the reader pane is empty', async () => {
    const wrapper = await mountFeeds()

    expect(wrapper.find('.panel-empty').text()).toContain('Select a post')
    expect(wrapper.find('.panel-empty').text()).toContain('Click a post in the list to view it here.')

    await wrapper.find('.item-row').trigger('click')
    await flushPromises()

    expect(wrapper.find('.panel-empty').exists()).toBe(false)
  })

  it('keeps the add dialog open when the backdrop is clicked', async () => {
    const wrapper = await mountFeeds()
    await flushPromises()

    await wrapper.find('.primary-action').trigger('click')
    expect(wrapper.find('.feed-dialog').exists()).toBe(true)

    await wrapper.find('.feed-dialog-overlay').trigger('click')
    expect(wrapper.find('.feed-dialog').exists()).toBe(true)
  })

  it('checks a pasted feed URL and fills the detected name', async () => {
    const wrapper = await mountFeeds()
    await flushPromises()

    await wrapper.find('.primary-action').trigger('click')
    await wrapper.find('#feed-url').setValue(feed.url)
    await wrapper.find('.detect-row .secondary-action').trigger('click')
    await flushPromises()

    expect(checkFeed).toHaveBeenCalledWith(feed.url)
    expect((wrapper.find('#feed-name').element as HTMLInputElement).value).toBe(feed.title)
  })

  it('shows progress and a count after manually refreshing feed items', async () => {
    const wrapper = await mountFeeds()
    await flushPromises()

    let resolveRefresh: (items: FeedItem[]) => void = () => {}
    vi.mocked(listFeedItems).mockImplementationOnce(
      () =>
        new Promise<FeedItem[]>((resolve) => {
          resolveRefresh = resolve
        }),
    )

    const refreshButton = wrapper.find('.refresh-action')
    await refreshButton.trigger('click')

    expect((refreshButton.element as HTMLButtonElement).disabled).toBe(true)
    expect(wrapper.find('.refresh-icon--spinning').exists()).toBe(true)

    resolveRefresh([item, secondItem])
    await flushPromises()

    expect((refreshButton.element as HTMLButtonElement).disabled).toBe(false)
    expect(wrapper.find('.feed-snackbar').text()).toBe('2 items found.')

    wrapper.unmount()
  })

  it('says nothing was found after a manual refresh returns no feed items', async () => {
    const wrapper = await mountFeeds()
    await flushPromises()

    vi.mocked(listFeedItems).mockResolvedValueOnce([])

    await wrapper.find('.refresh-action').trigger('click')
    await flushPromises()

    expect(wrapper.find('.feed-snackbar').text()).toBe('Nothing was found.')

    wrapper.unmount()
  })

  it('marks an opened unread item as read after five seconds', async () => {
    vi.useFakeTimers()
    const wrapper = await mountFeeds()
    await flushPromises()

    await wrapper.find('.item-row').trigger('click')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()

    expect(updateFeedItem).toHaveBeenCalledWith('item-1', { read: true })
    expect(wrapper.find('.item-row').classes()).toContain('item-row--read-dimmed')
  })

  it('starts the read dwell timer when the inbox item is clicked', async () => {
    vi.useFakeTimers()
    vi.mocked(getFeedItem).mockImplementationOnce(
      () =>
        new Promise<FeedItem>((resolve) => {
          setTimeout(() => resolve(item), 4000)
        }),
    )
    const wrapper = await mountFeeds()
    await flushPromises()

    await wrapper.find('.item-row').trigger('click')
    await vi.advanceTimersByTimeAsync(4999)
    await flushPromises()

    expect(updateFeedItem).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(1)
    await flushPromises()

    expect(updateFeedItem).toHaveBeenCalledWith('item-1', { read: true })

    wrapper.unmount()
  })

  it('can show all items for an individual feed', async () => {
    const wrapper = await mountFeeds()
    await flushPromises()

    vi.mocked(listFeedItems).mockClear()
    await wrapper.find('.feed-main').trigger('click')
    await flushPromises()

    expect(listFeedItems).toHaveBeenLastCalledWith('unread', 'feed-1')
    await wrapper.find('.feed-view-toggle input').setValue(true)
    await flushPromises()

    expect(listFeedItems).toHaveBeenLastCalledWith('all', 'feed-1')
  })

  it('marks all unread items for an individual feed as read', async () => {
    const wrapper = await mountFeeds()
    await flushPromises()

    vi.mocked(listFeedItems).mockClear()
    await wrapper.find('.feed-main').trigger('click')
    await flushPromises()

    const markButton = wrapper.find('.mark-read-action')
    expect(markButton.exists()).toBe(true)

    await markButton.trigger('click')
    await flushPromises()

    expect(markFeedRead).toHaveBeenCalledWith('feed-1')
    expect(listFeedItems).toHaveBeenLastCalledWith('unread', 'feed-1')
    expect(wrapper.find('.feed-snackbar').text()).toBe('Marked 1 item read.')
  })

  it('confirms before deleting a feed and refreshes the inbox', async () => {
    const wrapper = await mountFeeds()
    await flushPromises()

    await wrapper.find('.feed-settings').trigger('click')
    expect(wrapper.find('.feed-dialog').exists()).toBe(true)

    await wrapper.find('.dialog-delete-action').trigger('click')
    expect(deleteFeed).not.toHaveBeenCalled()
    expect(wrapper.find('.delete-confirmation').text()).toContain('Delete Example Feed?')

    await wrapper.find('.delete-confirmation .danger-action').trigger('click')
    await flushPromises()

    expect(deleteFeed).toHaveBeenCalledWith('feed-1')
    expect(wrapper.find('.feed-dialog').exists()).toBe(false)
    expect(wrapper.find('.feed-snackbar').text()).toBe('Feed deleted.')
    expect(listFeeds).toHaveBeenCalledTimes(2)
    expect(listFeedItems).toHaveBeenLastCalledWith('unread', null)
  })

  it('shows an active yellow filled star after starring an item', async () => {
    vi.mocked(updateFeedItem).mockResolvedValueOnce({ ...item, starred: true })
    const wrapper = await mountFeeds()
    await flushPromises()

    await wrapper.find('.item-row').trigger('click')
    await flushPromises()
    await wrapper.find('.star-action').trigger('click')
    await flushPromises()

    expect(updateFeedItem).toHaveBeenCalledWith('item-1', { starred: true })
    expect(wrapper.find('.star-action--active').exists()).toBe(true)
    expect(wrapper.find('.star-action').attributes('aria-pressed')).toBe('true')
    expect(wrapper.find('.item-star-icon').exists()).toBe(true)
  })

  it('shows the summary shimmer while requesting a sized feed description', async () => {
    let resolveSummary: (value: FeedItem) => void = () => {}
    vi.mocked(summarizeFeedItem).mockImplementationOnce(
      () =>
        new Promise<FeedItem>((resolve) => {
          resolveSummary = resolve
        }),
    )
    const wrapper = await mountFeeds()
    await flushPromises()

    await wrapper.find('.item-row').trigger('click')
    await flushPromises()
    await wrapper.find('button[title="Summarize"]').trigger('click')
    await flushPromises()

    expect(summarizeFeedItem).toHaveBeenCalledWith('item-1', 'summary', 150)
    expect(wrapper.find('.summary-working-text').text()).toBe('Summarizing...')
    expect(wrapper.find('.item-preview--summarizing').exists()).toBe(true)

    resolveSummary({ ...item, summary: 'An AI-sized preview.', summaryStatus: 'ready' })
    await flushPromises()

    expect(wrapper.find('.item-preview').text()).toBe('An AI-sized preview.')

    wrapper.unmount()
  })

  it('disables summary controls for video items', async () => {
    vi.mocked(getFeedItem).mockResolvedValueOnce({
      ...item,
      mediaType: 'youtube',
      mediaUrl: 'https://www.youtube.com/watch?v=abc123',
    })
    const wrapper = await mountFeeds()
    await flushPromises()

    await wrapper.find('.item-row').trigger('click')
    await flushPromises()

    const summarizeButton = wrapper.find('button[title="Summary unavailable for video items"]')
    expect(summarizeButton.exists()).toBe(true)
    expect((summarizeButton.element as HTMLButtonElement).disabled).toBe(true)
    await summarizeButton.trigger('click')
    expect(summarizeFeedItem).not.toHaveBeenCalled()
  })

  it('selects the feed from the URL query on load', async () => {
    const wrapper = await mountFeeds({ query: { feedId: 'feed-1' } })

    expect(wrapper.find('.feed-row--active').exists()).toBe(true)
    expect(wrapper.find('.feed-row--active .feed-row-title').text()).toBe('Example Feed')
    expect(listFeedItems).toHaveBeenCalledWith('unread', 'feed-1')
  })

  it('opens the item from postId query on load', async () => {
    const wrapper = await mountFeeds({
      query: { feedId: 'feed-1', postId: 'item-1' },
    })

    expect(wrapper.find('.item-row--selected').exists()).toBe(true)
    expect(wrapper.find('.panel-header h2').text()).toBe('First post')
  })
})
