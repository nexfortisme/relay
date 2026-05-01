import { createRouter, createWebHistory } from 'vue-router'

import ChatView from '../views/ChatView.vue'
import FeedsView from '../views/FeedsView.vue'
import HomeView from '../views/HomeView.vue'
import LoginView from '../views/LoginView.vue'
import MyDataView from '../views/MyDataView.vue'
import NotebooksView from '../views/NotebooksView.vue'
import RegisterView from '../views/RegisterView.vue'
import ScheduledView from '../views/ScheduledView.vue'
import { useAuthStore } from '../stores/authStore'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: LoginView, meta: { public: true } },
    { path: '/register', name: 'register', component: RegisterView, meta: { public: true } },
    { path: '/', name: 'home', component: HomeView },
    { path: '/chat', name: 'chat', component: ChatView },
    { path: '/notebooks', name: 'notebooks', component: NotebooksView },
    { path: '/scheduled', name: 'scheduled', component: ScheduledView },
    { path: '/my-data', name: 'my-data', component: MyDataView },
    { path: '/feeds', name: 'feeds', component: FeedsView },
  ],
})

// Global auth guard. We initialize the auth store lazily (first navigation)
// so that hard-refreshes still get a single /auth/me probe before routing
// rather than always rendering /login first and bouncing.
router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.initialized) {
    await auth.initialize()
  }
  const isPublic = to.meta?.public === true
  if (!auth.isAuthenticated && !isPublic) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (auth.isAuthenticated && isPublic) {
    return { path: '/' }
  }
  return true
})
