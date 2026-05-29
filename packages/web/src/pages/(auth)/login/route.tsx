import { createFileRoute } from '@tanstack/solid-router'

export const Route = createFileRoute('/(auth)/login')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Hello "/(app)/category"!</div>
}
