import { createFileRoute } from "@tanstack/solid-router";
import { postService } from "~/api";

export const Route = createFileRoute("/(app)/timeline/post")({
  component: RouteComponent,
});

function RouteComponent() {
  const { search } = Route.useSearch();
  const id = search.id as string | undefined;
  return <div>id: {id ?? "(none)"}</div>;
}
