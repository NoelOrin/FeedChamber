/// <reference types="vite/client" />
import { Scripts, createRootRoute } from '@tanstack/solid-router'
import { HydrationScript } from 'solid-js/web'
import type * as Solid from 'solid-js'
import Layout from '~/layouts/layout'
import styles from '~/styles.css?url'

export const Route = createRootRoute({
  head: () => ({
    meta: [
      { charset: 'utf-8' },
      {
        name: 'viewport',
        content: 'width=device-width, initial-scale=1.0',
      },
      { name: 'description', content: 'NoelOrin 的个人博客' },
    ],
    links: [
      { rel: 'stylesheet', href: styles },
      { rel: 'icon', href: '/favicon.ico' },
    ],
  }),
  shellComponent: RootDocument,
})

function DarkModeScript() {
  const code = `(function(){if(document.documentElement.classList.contains('dark'))return;var t=localStorage.getItem('theme');var d=(!t&&window.matchMedia('(prefers-color-scheme:dark)').matches)||t==='dark';if(d)document.documentElement.classList.add('dark')})();`
  return <script>{code}</script>
}

function RootDocument({ children }: { children: Solid.JSX.Element }) {
  return (
    <html lang="zh-CN">
      <head>
        <HydrationScript />
        <title>OrinForum</title>
        <DarkModeScript />
      </head>
      <body class="layout-default-bg">
        <Layout>
          {children}
        </Layout>
        <Scripts />
      </body>
    </html>
  )
}
