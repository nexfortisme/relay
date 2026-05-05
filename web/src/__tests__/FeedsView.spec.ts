import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import FeedsView from '../views/FeedsView.vue'
import {
  checkFeed,
  getFeedItem,
  listFeedItems,
  listFeeds,
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
    getFeedItem: vi.fn<typeof actual.getFeedItem>(),
    listFeedItems: vi.fn<typeof actual.listFeedItems>(),
    listFeeds: vi.fn<typeof actual.listFeeds>(),
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
  vi.mocked(checkFeed).mockResolvedValue({
    url: feed.url,
    title: feed.title,
    siteUrl: feed.siteUrl,
    description: '',
    itemCount: 1,
  })
}

function mountFeeds() {
  return mount(FeedsView, {
    global: {
      plugins: [createPinia()],
      stubs: {
        AppSidebar: true,
        PageNavTabs: true,
      },
    },
  })
}

describe('FeedsView', () => {
  beforeEach(() => {
    mockApi()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.clearAllMocks()
  })

  it('keeps the add dialog open when the backdrop is clicked', async () => {
    const wrapper = mountFeeds()
    await flushPromises()

    await wrapper.find('.primary-action').trigger('click')
    expect(wrapper.find('.feed-dialog').exists()).toBe(true)

    await wrapper.find('.feed-dialog-overlay').trigger('click')
    expect(wrapper.find('.feed-dialog').exists()).toBe(true)
  })

  it('checks a pasted feed URL and fills the detected name', async () => {
    const wrapper = mountFeeds()
    await flushPromises()

    await wrapper.find('.primary-action').trigger('click')
    await wrapper.find('#feed-url').setValue(feed.url)
    await wrapper.find('.detect-row .secondary-action').trigger('click')
    await flushPromises()

    expect(checkFeed).toHaveBeenCalledWith(feed.url)
    expect((wrapper.find('#feed-name').element as HTMLInputElement).value).toBe(feed.title)
  })

  it('shows progress and a count after manually refreshing feed items', async () => {
    const wrapper = mountFeeds()
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
    const wrapper = mountFeeds()
    await flushPromises()

    vi.mocked(listFeedItems).mockResolvedValueOnce([])

    await wrapper.find('.refresh-action').trigger('click')
    await flushPromises()

    expect(wrapper.find('.feed-snackbar').text()).toBe('Nothing was found.')

    wrapper.unmount()
  })

  it('marks an opened unread item as read after five seconds', async () => {
    vi.useFakeTimers()
    const wrapper = mountFeeds()
    await flushPromises()

    await wrapper.find('.item-row').trigger('click')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()

    expect(updateFeedItem).toHaveBeenCalledWith('item-1', { read: true })
  })

  it('can show all items for an individual feed', async () => {
    const wrapper = mountFeeds()
    await flushPromises()

    vi.mocked(listFeedItems).mockClear()
    await wrapper.find('.feed-main').trigger('click')
    await flushPromises()

    expect(listFeedItems).toHaveBeenLastCalledWith('unread', 'feed-1')
    await wrapper.find('.feed-view-toggle input').setValue(true)
    await flushPromises()

    expect(listFeedItems).toHaveBeenLastCalledWith('all', 'feed-1')
  })

  it('disables summary controls for video items', async () => {
    vi.mocked(getFeedItem).mockResolvedValueOnce({
      ...item,
      mediaType: 'youtube',
      mediaUrl: 'https://www.youtube.com/watch?v=abc123',
    })
    const wrapper = mountFeeds()
    await flushPromises()

    await wrapper.find('.item-row').trigger('click')
    await flushPromises()

    const summarizeButton = wrapper.find('button[title="Summary unavailable for video items"]')
    expect(summarizeButton.exists()).toBe(true)
    expect((summarizeButton.element as HTMLButtonElement).disabled).toBe(true)
    await summarizeButton.trigger('click')
    expect(summarizeFeedItem).not.toHaveBeenCalled()
  })
})
