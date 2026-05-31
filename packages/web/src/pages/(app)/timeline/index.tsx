import { createFileRoute } from "@tanstack/solid-router";
import { createSignal, For } from "solid-js";
import ScrollPaginator from "~/components/pagination/ScrollPaginator";
import ClientOnly from "~/components/ClientOnly";
import { postService } from "~/api";
import type { PostListItem } from "~/api";

const DEFAULT_PAGE_SIZE = 10;

export const Route = createFileRoute("/(app)/timeline/")({
  component: Home,
  loader: async () => {
    const items = await postService.list({
      page: 1,
      pageSize: DEFAULT_PAGE_SIZE,
    });
    return {
      items: items ?? [],
      hasMore: (items?.length ?? 0) >= DEFAULT_PAGE_SIZE,
    };
  },
});

function Home() {
  const loaderData = Route.useLoaderData();
  const [posts, setPosts] = createSignal<PostListItem[]>(loaderData().items);
  const [page, setPage] = createSignal(2);
  const [pageSize, setPageSize] = createSignal(DEFAULT_PAGE_SIZE);
  const [hasMore, setHasMore] = createSignal(loaderData().hasMore);
  const [isLoading, setIsLoading] = createSignal(false);

  const loadMore = async () => {
    if (isLoading() || !hasMore()) return;
    setIsLoading(true);
    try {
      const items =
        (await postService.list({ page: page(), pageSize: pageSize() })) ?? [];
      setPosts((prev) => [...prev, ...items]);
      setPage((p) => p + 1);
      if (items.length < pageSize()) setHasMore(false);
    } catch {
      setHasMore(false);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div class="w-full h-full flex flex-col">
      <div class="grid gap-4 lg:grid-cols-1 xl:grid-cols-2">
        <For each={posts()}>
          {(post) => (
            <div class="card p-4">
              <div class="font-bold text-lg">{post.title}</div>
              <div class="text-sm text-remark mt-1 line-clamp-2">
                {post.contentDigest}
              </div>
              <div class="mt-2 text-xs text-remark flex gap-3">
                <span>{post.authorName}</span>
                <span>{post.categoryName}</span>
                <span>{post.commentCount} 评论</span>
              </div>
            </div>
          )}
        </For>
      </div>
      <ClientOnly>
        <ScrollPaginator
          hasMore={hasMore()}
          isLoading={isLoading()}
          onLoadMore={loadMore}
        />
      </ClientOnly>
    </div>
  );
}
