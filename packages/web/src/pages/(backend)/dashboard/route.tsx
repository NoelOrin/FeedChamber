import { createFileRoute, Outlet } from '@tanstack/solid-router'

export const Route = createFileRoute('/(backend)/dashboard')({
  component: RouteComponent,
})

function RouteComponent() {
  return <Outlet />
}
