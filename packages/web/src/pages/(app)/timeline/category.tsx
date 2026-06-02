import { createFileRoute } from '@tanstack/solid-router'

export const Route = createFileRoute('/(app)/timeline/category')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Hello "/(app)/timeline/category"!</div>
}
