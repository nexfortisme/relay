import { createRouter, createWebHistory } from 'vue-router'

import ChatView from '../views/ChatView.vue'
import FeedsView from '../views/FeedsView.vue'
import HomeView from '../views/HomeView.vue'
import LoginView from '../views/LoginView.vue'
import MyDataView from '../views/MyDataView.vue'
import NotebookDetailView from '../views/NotebookDetailView.vue'
import NotebookEmptyView from '../views/NotebookEmptyView.vue'
import NotebooksView from '../views/NotebooksView.vue'
import ScheduledBuilderView from '../views/ScheduledBuilderView.vue'
import ScheduledView from '../views/ScheduledView.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/login', name: 'login', component: LoginView },
    { path: '/chat', name: 'chat', component: ChatView },
    { path: '/notebooks', name: 'notebooks', component: NotebooksView },
    { path: '/notebooks/new', name: 'notebooks-new', component: NotebookEmptyView },
    { path: '/notebooks/:id', name: 'notebook-detail', component: NotebookDetailView },
    { path: '/scheduled', name: 'scheduled', component: ScheduledView },
    { path: '/scheduled/new', name: 'scheduled-new', component: ScheduledBuilderView },
    { path: '/my-data', name: 'my-data', component: MyDataView },
    { path: '/feeds', name: 'feeds', component: FeedsView },
  ],
})
