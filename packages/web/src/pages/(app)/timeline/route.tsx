import { createFileRoute } from '@tanstack/solid-router'

export const Route = createFileRoute('/(app)/timeline')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Hello "/(app)/category"!</div>
}
