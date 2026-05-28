import { RouterProvider, createRouter } from '@tanstack/solid-router'
import { render } from 'solid-js/web'
import { routeTree } from './routeTree.gen'
import './styles.css'

const router = createRouter({ routeTree })

declare module '@tanstack/solid-router' {
  interface Register {
    router: typeof router
  }
}

const root = document.getElementById('app')
if (root) {
  render(() => <RouterProvider router={router} />, root)
}