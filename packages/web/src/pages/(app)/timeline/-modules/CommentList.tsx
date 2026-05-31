import { createSignal, For } from "solid-js"
import type { ContentListItem } from "~/api"
import { contentService } from "~/api"
import ScrollPaginator from "~/components/pagination/ScrollPaginator"
import CommentItem from "./CommentItem"

interface CommentListProps {
  postId: string
  initialComments: ContentListItem[]
  initialHasMore: boolean
  totalCount: number
}

export default function CommentList(props: CommentListProps) {
  const [comments, setComments] = createSignal<ContentListItem[]>(props.initialComments)
  const [hasMore, setHasMore] = createSignal(props.initialHasMore)
  const [isLoading, setIsLoading] = createSignal(false)

  const loadMore = async () => {
    if (isLoading() || !hasMore()) return
    setIsLoading(true)
    try {
      const items =
        (await contentService.list(props.postId, {
          page: Math.floor(comments().length / 20) + 1,
          pageSize: 20,
        })) ?? []
      setComments((prev) => [...prev, ...items])
      if (items.length < 20) setHasMore(false)
    } catch {
      setHasMore(false)
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div class="space-y-4">
      <h2 class="font-bold text-lg">{props.totalCount} 条评论</h2>
      <For each={comments()}>
        {(comment) => <CommentItem comment={comment} />}
      </For>
      <ScrollPaginator
        hasMore={hasMore()}
        isLoading={isLoading()}
        onLoadMore={loadMore}
      />
    </div>
  )
}
