import { createFileRoute, useNavigate, useSearch } from "@tanstack/solid-router";
import { createSignal, For, on, createEffect } from "solid-js";
import ScrollPaginator from "~/components/pagination/ScrollPaginator";
import { postService } from "~/api";
import type { PostListItem } from "~/api";

const DEFAULT_PAGE_SIZE = 10;

export const Route = createFileRoute("/(app)/timeline/")({
  component: Home,
  validateSearch: (search: Record<string, unknown>) => {
    const category = (search.category as string) || undefined;
    const tag = (search.tag as string) || undefined;
    return {
      category: category ? category : undefined,
      tag: !category && tag ? tag : undefined,
    };
  },
  loader: async ({ location }) => {
    const category = (location.search as Record<string, string>).category;
    const tag = (location.search as Record<string, string>).tag;
    const result = await postService.list({
      page: 1,
      pageSize: DEFAULT_PAGE_SIZE,
      categoryName: category,
      tagName: tag,
    });
    const p = result.pagination;
    return {
      items: result.data ?? [],
      hasMore: p ? p.currentPage < p.totalPages : false,
    };
  },
});

function Home() {
  const loaderData = Route.useLoaderData();
  const search = useSearch({ from: "/(app)/timeline/" });
  const [posts, setPosts] = createSignal<PostListItem[]>(loaderData().items);
  const [page, setPage] = createSignal(2);
  const [pageSize] = createSignal(DEFAULT_PAGE_SIZE);
  const [hasMore, setHasMore] = createSignal(loaderData().hasMore);
  const [isLoading, setIsLoading] = createSignal(false);

  createEffect(
    on(
      search,
      async (s) => {
        setIsLoading(true);
        try {
          const result = await postService.list({
            page: 1,
            pageSize: pageSize(),
            categoryName: s.category,
            tagName: s.tag,
          });
          const items = result.data ?? [];
          const p = result.pagination;
          setPosts(items);
          setPage(2);
          setHasMore(p ? p.currentPage < p.totalPages : false);
        } catch {
          setHasMore(false);
        } finally {
          setIsLoading(false);
        }
      },
      { defer: true },
    ),
  );

  const loadMore = async () => {
    if (isLoading() || !hasMore()) return;
    setIsLoading(true);
    try {
      const s = search();
      const result = await postService.list({
        page: page(),
        pageSize: pageSize(),
        categoryName: s.category,
        tagName: s.tag,
      });
      const items = result.data ?? [];
      const p = result.pagination;
      setPosts((prev) => [...prev, ...items]);
      setPage((p) => p + 1);
      if (p && p.currentPage >= p.totalPages) setHasMore(false);
    } catch {
      setHasMore(false);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div class="w-full h-full flex flex-col pb-32">
      <div class="grid gap-2 grid-cols-1">
        <For each={posts()}>{(post) => <PostItem post={post} />}</For>
      </div>
      <ScrollPaginator
        hasMore={hasMore()}
        isLoading={isLoading()}
        onLoadMore={loadMore}
      />
    </div>
  );
}

const PostItem = ({ post }: { post: PostListItem }) => {
  const navigate = useNavigate();
  const navigateToPost = () =>
    navigate({ to: `/timeline/post`, search: { id: post.id } });
  return (
    <div class="card p-4 cursor-pointer" onClick={navigateToPost}>
      <div class="font-bold text-lg">{post.title}</div>
      <div class="text-sm text-remark mt-1 line-clamp-2">
        {post.content_digest}
      </div>
      <div class="mt-2 text-xs text-remark flex gap-3">
        <span>{post.author_name}</span>
        <span>{post.category_name}</span>
        <span>{post.comment_count} 评论</span>
      </div>
    </div>
  );
};
