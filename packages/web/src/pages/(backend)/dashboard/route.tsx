import { createFileRoute } from '@tanstack/solid-router'

export const Route = createFileRoute('/(backend)/dashboard')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Hello "/(backend)/dashboard"!</div>
}
