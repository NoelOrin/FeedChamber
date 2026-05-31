import { Show } from "solid-js";
import { createFileRoute } from "@tanstack/solid-router";
import { postService, contentService } from "~/api";
import type { ContentListItem, PostListItem } from "~/api";
import PostHeader from "./-modules/PostHeader";
import CommentList from "./-modules/CommentList";
import ContentItem from "./-modules/ContentItem";

export const Route = createFileRoute("/(app)/timeline/post")({
  component: RouteComponent,
  head: (props) => {
    const post = (props.loaderData as any)?.post;
    const title = post?.title ? `${post.title} - OrinForum` : "OrinForum";
    const description = post?.content_digest || "OrinForum 社区论坛";
    const url = typeof window !== "undefined" ? window.location.href : "";
    // console.log(props);
    // console.log(post, title, description, url);
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
    const { id } = location.search as Record<string, string>;
    const [post, contents] = await Promise.all([
      postService.getById(id),
      contentService.list(id, { page: 1, pageSize: 30 }),
    ]);
    return {
      post: post ?? ({} as PostListItem),
      contents: contents ?? [],
      hasMore: (contents?.length ?? 0) >= 30,
    };
  },
});

function RouteComponent() {
  const loaderData = Route.useLoaderData() as () => {
    post: PostListItem;
    contents: ContentListItem[];
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
          totalCount={post().comment_count}
        />
      </div>
    </div>
  );
}
