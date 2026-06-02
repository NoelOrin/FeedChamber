/// <reference types="vite/client" />
import { createRootRoute, HeadContent, Scripts, Outlet } from '@tanstack/solid-router'
import type * as Solid from 'solid-js'
import { HydrationScript } from 'solid-js/web'
import ErrorComponent from '~/components/ErrorComponent'
import NotFound from '~/components/NotFound'
import Layout from '~/layouts/layout'
import styles from '~/styles.css?url'
import { categoryService, tagService, settingService } from '~/api'

export const Route = createRootRoute({
  notFoundComponent: NotFound,
  errorComponent: ErrorComponent,
  loader: async () => {
    const [catResult, tagResult, settings] = await Promise.all([
      categoryService.list({ page: 1, pageSize: 50 }),
      tagService.list({ page: 1, pageSize: 50 }),
      settingService.getPublic(),
    ])
    return {
      categories: catResult.data ?? [],
      tags: tagResult.data ?? [],
      settings: settings ?? {},
    }
  },
  head: (props) => {
    const settings = (props.loaderData as any)?.settings ?? {};
    return {
      meta: [
        { charset: 'utf-8' },
        {
          name: 'viewport',
          content: 'width=device-width, initial-scale=1.0',
        },
        { name: 'description', content: settings.seo_description || settings.site_description || '' },
        ...(settings.seo_keywords ? [{ name: 'keywords', content: settings.seo_keywords }] : []),
        ...(settings.site_url ? [{ property: 'og:url', content: settings.site_url }] : []),
      ],
      links: [
        { rel: 'stylesheet', href: styles },
        { rel: 'icon', href: '/favicon.ico' },
      ],
    }
  },
  component: RootComponent,
  shellComponent: RootDocument,
})

function RootComponent() {
  return <Outlet />
}

function DarkModeScript() {
  const code = `(function(){var t=localStorage.getItem('theme');var m=t||'system';var isDark=m==='dark'||(m!=='light'&&window.matchMedia('(prefers-color-scheme:dark)').matches);document.documentElement.setAttribute('data-theme-mode',m);document.documentElement.classList.toggle('dark',isDark);document.documentElement.setAttribute('data-theme',isDark?'dark':'light')})();`
  return <script>{code}</script>
}

function RootDocument({ children }: { children: Solid.JSX.Element }) {
  const settings = Route.useLoaderData()().settings ?? {};
  const title = settings.seo_title || settings.site_name || 'OrinForum';

  return (
    <html lang="zh-CN">
      <head>
        <HeadContent />
        <HydrationScript />
        <title>{title}</title>
        <DarkModeScript />
      </head>
      <body class="layout-default-bg relative">
        <Layout>
          {children}
        </Layout>
        <Scripts />
      </body>
    </html>
  )
}
