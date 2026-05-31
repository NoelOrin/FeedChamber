import { createFileRoute, redirect } from "@tanstack/solid-router";

export const Route = createFileRoute("/(app)/")({
  beforeLoad: () => {
    throw redirect({
      to: "/timeline",
      search: { category: undefined, tag: undefined },
    });
  },
});

function RouteComponent() {
  return <div>Hello "/(app)/"!</div>;
}
