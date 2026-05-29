import { createRootRoute, Outlet } from '@tanstack/solid-router'
import { MetaProvider, Title } from '@solidjs/meta'
import Layout from '~/layouts/layout'

export const Route = createRootRoute({
  head: () => ({
    meta: [
      { name: 'description', content: 'NoelOrin 的个人博客' },
      { name: 'viewport', content: 'width=device-width, initial-scale=1.0' },
    ],
    title: 'OrinForum',
  }),
  component: RootComponent,
})

function RootComponent() {
  return (
    <MetaProvider>
      <Title>OrinForum</Title>
      <Layout>
        <Outlet />
      </Layout>
    </MetaProvider>
  )
}
