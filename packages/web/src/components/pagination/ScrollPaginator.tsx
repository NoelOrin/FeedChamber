import {
  createSignal,
  createEffect,
  onCleanup,
  Switch,
  Match,
  JSX,
} from "solid-js";

interface ScrollPaginatorProps {
  class?: string;
  hasMore: boolean;
  isLoading: boolean;
  onLoadMore: () => void;
  loadingComponent?: JSX.Element;
  threshold?: number;
}

export default function ScrollPaginator(props: ScrollPaginatorProps) {
  const [sentinelRef, setSentinelRef] = createSignal<HTMLDivElement | null>(
    null,
  );
  let observer: IntersectionObserver | null = null;

  const startObserving = () => {
    const sentinel = sentinelRef();
    if (!sentinel || !observer) return;
    observer.unobserve(sentinel);
    observer.observe(sentinel);
  };

  createEffect(() => {
    if (typeof IntersectionObserver === "undefined") return;

    observer = new IntersectionObserver(
      (entries) => {
        const entry = entries[0];
        if (!entry.isIntersecting) return;
        if (!props.hasMore || props.isLoading) return;
        // unobserve to prevent duplicate triggers while loading
        observer?.unobserve(entry.target);
        props.onLoadMore();
      },
      { threshold: 1, rootMargin: `-${props.threshold ?? 110}px` },
    );

    startObserving();
    onCleanup(() => observer?.disconnect());
  });

  // Re-observe after loading finishes or hasMore changes,
  // so the next batch can be triggered once all items are rendered.
  createEffect(() => {
    // reading these signals makes the effect re-run when they change
    props.isLoading;
    props.hasMore;
    const sentinel = sentinelRef();
    if (!sentinel || !observer) return;
    observer.unobserve(sentinel);
    if (!props.isLoading && props.hasMore) {
      // give the browser a frame to lay out new items before re-observing
      requestAnimationFrame(() => startObserving());
    }
  });

  return (
    <div class={`w-full min-h-[32px] ${props.class}`}>
      <div ref={setSentinelRef} class="py-2">
        <Switch>
          <Match when={props.isLoading}>
            {props.loadingComponent ?? <DefaultLoading />}
          </Match>
          <Match when={!props.hasMore}>
            <div class="text-center font-bold py-3">没有更多了</div>
          </Match>
        </Switch>
      </div>
    </div>
  );
}

const DefaultLoading = () => (
  <div class="flex items-center justify-centerpy-4 flex-col space-y-2">
    <div class="text-center font-bold">加载中...</div>
    <div class="loading loading-spinner loading-sm" />
  </div>
);
