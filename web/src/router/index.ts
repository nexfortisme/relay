import { createRouter, createWebHistory } from 'vue-router'

import ChatView from '../views/ChatView.vue'
import FeedsView from '../views/FeedsView.vue'
import HomeView from '../views/HomeView.vue'
import MyDataView from '../views/MyDataView.vue'
import NotebooksView from '../views/NotebooksView.vue'
import ScheduledView from '../views/ScheduledView.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/chat', name: 'chat', component: ChatView },
    { path: '/notebooks', name: 'notebooks', component: NotebooksView },
    { path: '/scheduled', name: 'scheduled', component: ScheduledView },
    { path: '/my-data', name: 'my-data', component: MyDataView },
    { path: '/feeds', name: 'feeds', component: FeedsView },
  ],
})
