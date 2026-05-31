/// <reference types="vite/client" />
import { createRootRoute, HeadContent, Scripts } from '@tanstack/solid-router'
import type * as Solid from 'solid-js'
import { HydrationScript } from 'solid-js/web'
import ErrorComponent from '~/components/ErrorComponent'
import NotFound from '~/components/NotFound'
import Layout from '~/layouts/layout'
import styles from '~/styles.css?url'

export const Route = createRootRoute({
  notFoundComponent: NotFound,
  errorComponent: ErrorComponent,
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
  const code = `(function(){var t=localStorage.getItem('theme');var d=t==='dark'||(t!=='light'&&window.matchMedia('(prefers-color-scheme:dark)').matches);if(d){document.documentElement.classList.add('dark');document.documentElement.setAttribute('data-theme','dark')}else{document.documentElement.setAttribute('data-theme','light')}})();`
  return <script>{code}</script>
}

function RootDocument({ children }: { children: Solid.JSX.Element }) {
  return (
    <html lang="zh-CN">
      <head>
        <HeadContent />
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
