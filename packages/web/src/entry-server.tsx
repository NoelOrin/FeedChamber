import { renderToString } from 'solid-js/web'
import { MetaProvider } from '@solidjs/meta'
import { createRouter, RouterProvider } from '@tanstack/solid-router'
import { createMemoryHistory } from '@tanstack/solid-router'
import { routeTree } from './routeTree.gen'
import Layout from './layouts/layout'

export async function render(url: string) {
  const router = createRouter({
    routeTree,
    defaultPreload: 'intent',
    scrollRestoration: true,
    history: createMemoryHistory({
      initialEntries: [url],
    }),
  })

  await router.load()

  let html = ''

  const App = () => (
    <MetaProvider>
      <Layout>
        <RouterProvider router={router} />
      </Layout>
    </MetaProvider>
  )

  html = renderToString(() => <App />)

  return {
    html,
    router,
  }
}
