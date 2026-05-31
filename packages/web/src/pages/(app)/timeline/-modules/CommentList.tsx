import { createSignal, For } from "solid-js";
import type { ContentListItem } from "~/api";
import { contentService } from "~/api";
import ScrollPaginator from "~/components/pagination/ScrollPaginator";
import ContentItem from "./ContentItem";

interface CommentListProps {
  postId: string;
  initialContents: ContentListItem[];
  initialHasMore: boolean;
  totalCount: number;
}

export default function CommentList(props: CommentListProps) {
  const [contents, setContents] = createSignal<ContentListItem[]>(
    props.initialContents,
  );
  const [hasMore, setHasMore] = createSignal(props.initialHasMore);
  const [isLoading, setIsLoading] = createSignal(false);

  const loadMore = async () => {
    if (isLoading() || !hasMore()) return;
    setIsLoading(true);
    try {
      const items =
        (await contentService.list(props.postId, {
          page: Math.floor(contents().length / 20) + 1,
          pageSize: 20,
        })) ?? [];
      setContents((prev) => [...prev, ...items]);
      if (items.length < 20) setHasMore(false);
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
