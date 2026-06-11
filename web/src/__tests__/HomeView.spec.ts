import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import HomeView from '../views/HomeView.vue'
import { listFeeds } from '../lib/api'
import { listNotebooks, type Notebook } from '../lib/notebooks'

vi.mock('../lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/api')>()
  return {
    ...actual,
    listFeeds: vi.fn<typeof actual.listFeeds>(),
  }
})

vi.mock('../lib/notebooks', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/notebooks')>()
  return {
    ...actual,
    listNotebooks: vi.fn<typeof actual.listNotebooks>(),
  }
})

const recentNotebook: Notebook = {
  id: 'notebook-1',
  name: 'Field Notes',
  description: 'Observations from the current project',
  systemPrompt: '',
  skillPrompt: '',
  includeInGeneral: false,
  pendingJobs: 2,
  createdAt: '2026-05-01T12:00:00.000Z',
  updatedAt: '2026-05-08T12:00:00.000Z',
}

const olderNotebook: Notebook = {
  ...recentNotebook,
  id: 'notebook-2',
  name: 'Old Research',
  description: '',
  pendingJobs: 0,
  createdAt: '2026-04-01T12:00:00.000Z',
  updatedAt: '2026-04-03T12:00:00.000Z',
}

async function mountHome() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/home', name: 'home', component: { template: '<div />' } },
      { path: '/notebooks', name: 'notebooks', component: { template: '<div />' } },
      { path: '/feeds', name: 'feeds', component: { template: '<div />' } },
      { path: '/chat', name: 'chat', component: { template: '<div />' } },
    ],
  })
  await router.push('/home')
  await router.isReady()

  const wrapper = mount(HomeView, {
    global: {
      plugins: [createPinia(), router],
      stubs: {
        AppSidebar: true,
      },
    },
  })
  await flushPromises()
  return wrapper
}

describe('HomeView', () => {
  beforeEach(() => {
    vi.mocked(listFeeds).mockResolvedValue([])
    vi.mocked(listNotebooks).mockResolvedValue([olderNotebook, recentNotebook])
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('lists the notebooks the user currently has', async () => {
    const wrapper = await mountHome()

    expect(listNotebooks).toHaveBeenCalledTimes(1)
    expect(wrapper.find('.home-card').text()).toContain('2 notebooks')
    expect(wrapper.findAll('.notebook-list-title').map((node) => node.text())).toEqual([
      'Field Notes',
      'Old Research',
    ])
    expect(wrapper.text()).toContain('Observations from the current project')
    expect(wrapper.text()).toContain('2 indexing jobs')
    expect(wrapper.text()).not.toContain('Cooking')
  })
})
