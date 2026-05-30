import { createRouter } from '@tanstack/solid-router'
import { routeTree } from './routeTree.gen'
import './styles.css'

declare module '@tanstack/solid-router' {
  interface Register {
    router: ReturnType<typeof getRouter>
  }
}

export function getRouter() {
  const router = createRouter({
    routeTree,
    defaultPreload: 'intent',
    scrollRestoration: true,
  })
  return router
}