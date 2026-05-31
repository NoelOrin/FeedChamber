import { Show } from "solid-js";
import { createFileRoute } from "@tanstack/solid-router";
import { postService, contentService } from "~/api";
import type { ContentListItem, PostListItem } from "~/api";
import PostHeader from "./-modules/PostHeader";
import PostContent from "./-modules/PostContent";
import CommentList from "./-modules/CommentList";

export const Route = createFileRoute("/(app)/timeline/post")({
  component: RouteComponent,
  loader: async ({ location }) => {
    const { id } = location.search as Record<string, string>;
    const [post, contents] = await Promise.all([
      postService.getById(id),
      contentService.list(id, { page: 1, pageSize: 20 }),
    ]);

    return {
      post: post ?? ({} as PostListItem),
      contents: contents ?? [],
      hasMore: (contents?.length ?? 0) >= 20,
    };
  },
});

function RouteComponent() {
  const loaderData = Route.useLoaderData();
  const post = () => loaderData().post as PostListItem;
  const contents = () => loaderData().contents as ContentListItem[];
    console.log(post(), contents());
    console.log(contents()[0]);
  return (
    <div class="w-full px-4 py-6 card">
      <PostHeader post={post()} />
      <Show when={contents().length > 0}>
        <PostContent body={contents()[0].body} />
      </Show>
      <div class="divider"></div>
      <CommentList
        postId={post().id}
        initialComments={contents().slice(1)}
        initialHasMore={loaderData().hasMore}
        totalCount={post().comment_count}
      />
    </div>
  );
}
