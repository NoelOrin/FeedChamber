import { createSignal, createEffect, onCleanup, Show, JSX } from "solid-js";

interface ScrollPaginatorProps {
//   children: JSX.Element;
  hasMore: boolean;
  isLoading: boolean;
  onLoadMore: () => void;
  loadingComponent?: JSX.Element;
  threshold?: number;
}

export default function ScrollPaginator(props: ScrollPaginatorProps) {
  const [sentinelRef, setSentinelRef] = createSignal<HTMLDivElement | null>(null);
  const threshold = () => props.threshold ?? 100;

  createEffect(() => {
    const sentinel = sentinelRef();
    if (!sentinel) return;

    const observer = new IntersectionObserver(
      (entries) => {
        const entry = entries[0];
        if (entry.isIntersecting && props.hasMore && !props.isLoading) {
          props.onLoadMore();
        }
      },
      {
        rootMargin: `${threshold()}px`,
      }
    );

    observer.observe(sentinel);

    onCleanup(() => observer.disconnect());
  });

  const defaultLoading = (
    <div class="flex justify-center py-4">
      <div class="loading loading-spinner loading-sm" />
    </div>
  );

  return (
    <div>
      <div ref={setSentinelRef} class="py-4">
        <Show when={props.isLoading}>{props.loadingComponent ?? defaultLoading}</Show>
        <Show when={!props.hasMore && !props.isLoading}>
          <div class="text-center text-gray-400 py-4">没有更多了</div>
        </Show>
      </div>
    </div>
  );
}