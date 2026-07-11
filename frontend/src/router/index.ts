// Composables
import { createRouter, createWebHistory } from 'vue-router'
import Login from '@/views/Login.vue'
import Data from '@/store/modules/data'

const routes = [
  {
    path: '/login',
    name: 'pages.login',
    component: Login,
  },
  {
    path: '/',
    component: () => import('@/layouts/default/Default.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: '/',
        name: 'pages.home',
        component: () => import('@/views/Home.vue'),
      },
      {
        path: '/config',
        name: 'pages.config',
        component: () => import('@/views/Config.vue'),
      },
      {
        path: '/clients',
        name: 'pages.clients',
        component: () => import('@/views/Clients.vue'),
      },
      {
        path: '/admins',
        redirect: '/settings?tab=admin',
      },
      {
        path: '/settings',
        name: 'pages.settings',
        component: () => import('@/views/Settings.vue'),
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory((window as any).BASE_URL),
  routes,
})

const DEFAULT_TITLE = 'S-UI'
let intervalId:any

// Navigation guard to check authentication state
router.beforeEach((to, from, next) => {
  // Check the session cookie
  const sessionCookie = document.cookie.split(';').find(cookie => cookie.trim().startsWith('s-ui='))
  const isAuthenticated = !!sessionCookie

  // If the route requires authentication and the user is not authenticated, redirect to /login
  if (to.meta.requiresAuth && !isAuthenticated) {
    next('/login')
  } else if (to.path === '/login' && isAuthenticated) {
    // If already authenticated and visiting /route, redirect to '/'
    next('/')
  } else {
    // Load default data
    if(to.path != '/login'){
      loadDataInterval()
    } else {
      if (intervalId) {
        clearInterval(intervalId)
        intervalId = undefined
      }
    }
    next()
  }
})

const loadDataInterval = () => {
  if (intervalId) return
  Data().loadData()
  intervalId = setInterval(() => {
    Data().loadData()
  }, 10000)
}

export default router
