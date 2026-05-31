import { createFileRoute, redirect } from "@tanstack/solid-router";

export const Route = createFileRoute("/(app)/")({
  beforeLoad: () => {
    throw redirect({
      to: "/timeline",
    });
  },
});

function RouteComponent() {
  return <div>Hello "/(app)/"!</div>;
}
