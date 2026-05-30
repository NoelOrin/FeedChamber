import { createFileRoute, Outlet } from '@tanstack/solid-router'

export const Route = createFileRoute('/(app)/timeline')({
  component: RouteComponent,
})

function RouteComponent() {
  return <Outlet />
}
