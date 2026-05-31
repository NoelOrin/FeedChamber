import { createFileRoute } from "@tanstack/solid-router"
import { createSignal, For } from "solid-js"
import ScrollPaginator from "~/components/pagination/ScrollPaginator"
import { postService } from "~/api"
import type { PostListItem } from "~/api"

export const Route = createFileRoute("/(app)/timeline/")({
  component: Home,
})

const PAGE_SIZE = 20

function Home() {
  const [posts, setPosts] = createSignal<PostListItem[]>([])
  const [page, setPage] = createSignal(1)
  const [hasMore, setHasMore] = createSignal(true)
  const [isLoading, setIsLoading] = createSignal(false)

  const loadMore = async () => {
    if (isLoading() || !hasMore()) return
    setIsLoading(true)
    try {
      const items = (await postService.list({ page: page(), pageSize: PAGE_SIZE })) ?? []
      setPosts((prev) => [...prev, ...items])
      setPage((p) => p + 1)
      if (items.length < PAGE_SIZE) setHasMore(false)
    } catch {
      setHasMore(false)
    } finally {
      setIsLoading(false)
    }
  }

  loadMore()

  return (
    <div class="w-full h-full flex flex-col">
      <div class="grid gap-4 lg:grid-cols-1 xl:grid-cols-2">
        <For each={posts()}>
          {(post) => (
            <div class="card p-4">
              <div class="font-bold text-lg">{post.title}</div>
              <div class="text-sm text-remark mt-1 line-clamp-2">{post.contentDigest}</div>
              <div class="mt-2 text-xs text-remark flex gap-3">
                <span>{post.authorName}</span>
                <span>{post.categoryName}</span>
                <span>{post.commentCount} 评论</span>
              </div>
            </div>
          )}
        </For>
      </div>
      <ScrollPaginator
        hasMore={hasMore()}
        isLoading={isLoading()}
        onLoadMore={loadMore}
      />
    </div>
  )
}
