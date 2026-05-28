import { createRootRoute, Outlet } from '@tanstack/solid-router'
import { MetaProvider, Title } from '@solidjs/meta'
import Layout from '~/layouts/layout'

export const Route = createRootRoute({
  component: () => (
    <MetaProvider>
      <Title>OrinForum</Title>
      <Layout>
        <Outlet />
      </Layout>
    </MetaProvider>
  ),
})