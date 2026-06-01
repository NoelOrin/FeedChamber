import { createSignal, For } from "solid-js";
import type { ContentListItem } from "~/api";
import { contentService } from "~/api";
import ScrollPaginator from "~/components/pagination/ScrollPaginator";
import ContentItem from "./ContentItem";

interface CommentListProps {
  postId: string;
  initialContents: ContentListItem[];
  initialHasMore: boolean;
  initialTotal: number;
  totalCount: number;
}

export default function CommentList(props: CommentListProps) {
  const [contents, setContents] = createSignal<ContentListItem[]>(
    props.initialContents,
  );
  const [page, setPage] = createSignal(2);
  const [total, setTotal] = createSignal(props.initialTotal);
  const [hasMore, setHasMore] = createSignal(props.initialHasMore);
  const [isLoading, setIsLoading] = createSignal(false);

  const PAGE_SIZE = 20;

  const loadMore = async () => {
    if (isLoading() || !hasMore()) return;
    setIsLoading(true);
    try {
      const result = await contentService.list(props.postId, {
        page: page(),
        pageSize: PAGE_SIZE,
      });
      const items = result.data ?? [];
      setContents((prev) => [...prev, ...items]);
      setPage((v) => v + 1);
      setTotal(result.total);
      if (contents().length >= result.total) setHasMore(false);
    } catch {
      setHasMore(false);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div class="space-y-4">
      <For each={contents()}>
        {(content) => <ContentItem content={content} />}
      </For>
      <ScrollPaginator
        hasMore={hasMore()}
        isLoading={isLoading()}
        onLoadMore={loadMore}
      />
    </div>
  );
}
