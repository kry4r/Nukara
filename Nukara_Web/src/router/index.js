import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/auth',
    name: 'Auth',
    component: () => import('../views/AuthView.vue'),
    meta: { guest: true },
  },
  {
    path: '/',
    name: 'Conversations',
    component: () => import('../views/ConversationsView.vue'),
    meta: { auth: true },
  },
  {
    path: '/chat/:convId',
    name: 'Chat',
    component: () => import('../views/ChatView.vue'),
    meta: { auth: true },
  },
  {
    path: '/bots',
    name: 'Bots',
    component: () => import('../views/BotsView.vue'),
    meta: { auth: true },
  },
  {
    path: '/bots/new',
    name: 'BotCreate',
    component: () => import('../views/BotFormView.vue'),
    meta: { auth: true },
  },
  {
    path: '/bots/:id/edit',
    name: 'BotEdit',
    component: () => import('../views/BotFormView.vue'),
    meta: { auth: true },
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import('../views/SettingsView.vue'),
    meta: { auth: true },
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
  const token = localStorage.getItem('nukara_token')
  if (to.meta.auth && !token) return '/auth'
  if (to.meta.guest && token) return '/'
})

export default router
