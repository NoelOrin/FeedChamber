import { Show } from "solid-js";
import { createFileRoute } from "@tanstack/solid-router";
import { postService, contentService, settingService } from "~/api";
import type { ContentListItem, PostListItem } from "~/api";
import PostHeader from "./-modules/PostHeader";
import CommentList from "./-modules/CommentList";
import ContentItem from "./-modules/ContentItem";

export const Route = createFileRoute("/(app)/timeline/post")({
  component: RouteComponent,
  validateSearch: (search: Record<string, unknown>) => ({
    id: (search.id as string) || "",
  }),
  loaderDeps: ({ search: { id } }) => ({ id }),
  staleTime: 0,
  pendingMs: 0,
  pendingMinMs: 0,
  pendingComponent: Pending,
  head: (props) => {
    const loaderData = props.loaderData as any;
    const post = loaderData?.post;
    const settings = loaderData?.settings ?? {};
    const siteName = settings.site_name || "OrinForum";
    const siteDesc = settings.site_description || "OrinForum 社区论坛";
    const title = post?.title ? `${post.title} - ${siteName}` : siteName;
    const description = post?.content_digest || siteDesc;
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

  loader: async ({ deps: { id } }) => {
    const [post, contentsResult, settings] = await Promise.all([
      postService.getById(id),
      contentService.list(id, { page: 1, pageSize: 30 }),
      settingService.getPublic(),
    ]);
    const contents = contentsResult.data ?? [];
    const total = contentsResult.pagination?.total ?? 0;
    return {
      post: post ?? ({} as PostListItem),
      contents,
      total,
      hasMore: contents.length < total,
      settings: settings ?? {},
    };
  },
});

function Pending() {
  return <div class="w-full flex justify-center py-12 text-remark">加载中...</div>;
}

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
