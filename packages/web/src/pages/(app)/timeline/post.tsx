import { Show } from "solid-js";
import { createFileRoute } from "@tanstack/solid-router";
import { postService, contentService } from "~/api";
import type { ContentListItem, PostListItem } from "~/api";
import PostHeader from "./-modules/PostHeader";
import CommentList from "./-modules/CommentList";
import ContentItem from "./-modules/ContentItem";

export const Route = createFileRoute("/(app)/timeline/post")({
  component: RouteComponent,
  validateSearch: (search: Record<string, unknown>) => ({
    id: (search.id as string) || "",
  }),
  head: (props) => {
    const post = (props.loaderData as any)?.post;
    const title = post?.title ? `${post.title} - OrinForum` : "OrinForum";
    const description = post?.content_digest || "OrinForum 社区论坛";
    const url = typeof window !== "undefined" ? window.location.href : "";
    return {
      meta: [
        { title },
        { name: "description", content: description },
        { property: "og:title", content: title },
        { property: "og:description", content: description },
        { property: "og:type", content: "article" },
        ...(url ? [{ property: "og:url", content: url }] : []),
        { name: "twitter:card", content: "summary" },
        { name: "twitter:title", content: title },
        { name: "twitter:description", content: description },
      ],
    };
  },

  loader: async ({ location }) => {
    const { id } = location.search as { id: string };
    const [post, contentsResult] = await Promise.all([
      postService.getById(id),
      contentService.list(id, { page: 1, pageSize: 30 }),
    ]);
    const contents = contentsResult.data ?? [];
    const total = contentsResult.total;
    return {
      post: post ?? ({} as PostListItem),
      contents,
      total,
      hasMore: contents.length < total,
    };
  },
});

function RouteComponent() {
  const loaderData = Route.useLoaderData() as () => {
    post: PostListItem;
    contents: ContentListItem[];
    total: number;
    hasMore: boolean;
  };
  const post = () => loaderData().post;
  const contents = () => loaderData().contents;
  return (
    <div class="w-full flex">
      <div class="w-full px-4 py-6 card">
        <PostHeader post={post()} />
        <Show when={contents().length > 0}>
          {/* <PostContent body={contents()[0].body} /> */}
          <ContentItem class="pt-6" content={contents()[0]} />
        </Show>
        <CommentList
          postId={post().id}
          initialContents={contents().slice(1)}
          initialHasMore={loaderData().hasMore}
          initialTotal={loaderData().total}
          totalCount={post().comment_count}
        />
      </div>
    </div>
  );
}
