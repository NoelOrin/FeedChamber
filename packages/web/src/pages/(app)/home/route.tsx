import { createFileRoute } from '@tanstack/solid-router'

export const Route = createFileRoute('/(app)/home')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Hello "/(app)/category"!</div>
}
