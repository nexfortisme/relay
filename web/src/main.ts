import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import { registerPrimitives } from './components/primitives'
import { router } from './router'

const app = createApp(App).use(createPinia()).use(router)
registerPrimitives(app)
app.mount('#app')
