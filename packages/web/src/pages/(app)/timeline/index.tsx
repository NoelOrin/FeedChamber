import { createFileRoute, useNavigate, useSearch } from "@tanstack/solid-router";
import { createSignal, For, createEffect, on } from "solid-js";
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
  loaderDeps: ({ search: { category, tag } }) => ({ category, tag }),
  loader: async ({ deps }) => {
    const result = await postService.list({
      page: 1,
      pageSize: DEFAULT_PAGE_SIZE,
      categoryName: deps.category,
      tagName: deps.tag,
    });
    const items = result.data ?? [];
    return {
      items,
      total: result.pagination?.total ?? 0,
      hasMore: items.length < (result.pagination?.total ?? 0),
    };
  },
});

function Home() {
  const loaderData = Route.useLoaderData();
  const search = useSearch({ from: "/(app)/timeline/" });
  const deps = Route.useLoaderDeps();
  const [posts, setPosts] = createSignal<PostListItem[]>(loaderData().items);
  const [page, setPage] = createSignal(2);
  const [pageSize] = createSignal(DEFAULT_PAGE_SIZE);
  const [total, setTotal] = createSignal(loaderData().total);
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
          setPosts(items);
          setPage(2);
          setTotal(result.pagination?.total ?? 0);
          setHasMore(items.length < (result.pagination?.total ?? 0));
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
      const result = await postService.list({
        page: page(),
        pageSize: DEFAULT_PAGE_SIZE,
        categoryName: deps().category,
        tagName: deps().tag,
      });
      const items = result.data ?? [];
      setPosts((prev) => [...prev, ...items]);
      setPage((p) => p + 1);
      if (posts().length >= total()) setHasMore(false);
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
