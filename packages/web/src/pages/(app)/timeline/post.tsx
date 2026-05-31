import {
  createFileRoute,
  useLoaderData,
  useLocation,
} from "@tanstack/solid-router";
import { postService } from "~/api";

export const Route = createFileRoute("/(app)/timeline/post")({
  component: RouteComponent,
    loader: async (params) => {
      console.log(params);
    const location = useLocation();
    const { id } = location().search as Record<string, string>;
    console.log(id);
    const items = await postService.getById(id);
    return {
      post: items ?? {},
    };
  },
});

function RouteComponent() {
    const loaderData = Route.useLoaderData();
    const { post } = loaderData();
    console.log(post);
  return <div>Hello "/(app)/timeline/post?id=${post.id || ""}!</div>;
}
