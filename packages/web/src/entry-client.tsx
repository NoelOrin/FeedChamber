/// <reference types="vite/client" />
import { hydrate } from 'solid-js/web'
import { MetaProvider } from '@solidjs/meta'
import { RouterProvider, createRouter } from '@tanstack/solid-router'
import { routeTree } from './routeTree.gen'
import Layout from './layouts/layout'
import './styles.css'

const router = createRouter({
  routeTree,
  defaultPreload: 'intent',
})

declare module '@tanstack/solid-router' {
  interface Register {
    router: typeof router
  }
}

hydrate(
  () => (
    <MetaProvider>
      <Layout>
        <RouterProvider router={router} />
      </Layout>
    </MetaProvider>
  ),
  document.getElementById('app')!
)
