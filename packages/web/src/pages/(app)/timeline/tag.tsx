import { createFileRoute } from '@tanstack/solid-router'

export const Route = createFileRoute('/(app)/timeline/tag')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Hello "/(app)/timeline/tag"!</div>
}
