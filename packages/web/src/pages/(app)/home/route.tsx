import { createFileRoute, Outlet } from '@tanstack/solid-router'

export const Route = createFileRoute('/(app)/home')({
  component: RouteComponent,
})

function RouteComponent() {
  return <Outlet />
}
