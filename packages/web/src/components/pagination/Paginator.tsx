import { createSignal, For, Show, createEffect, createMemo } from "solid-js";
import { Link, useLocation } from "@tanstack/solid-router";

interface PaginatorProps {
  pageNum: number;
  pageSize: number;
  total: number;
}

export default function Paginator(props: PaginatorProps) {
  const totalPages = () => Math.ceil(props.total / props.pageSize) || 1;
  const [currentPage, setCurrentPage] = createSignal(1);
  const location = useLocation();
  const page = createMemo(
    () => Number((location().search as Record<string, string>)?.page) || 1,
  );

  createEffect(() => {
    setCurrentPage(page());
  });

  const scrollToTop = () => {
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  const renderPageLinks = createMemo(() => {
    const pages: number[] = [];
    const current = currentPage();
    const total = totalPages();

    if (total <= 11) {
      for (let i = 1; i <= total; i++) pages.push(i);
      return pages;
    }

    let start = current - 5;
    let end = current + 5;
    if (start < 1) {
      start = 1;
      end = 11;
    } else if (end > total) {
      start = total - 10;
      end = total;
    }

    for (let i = start; i <= end; i++) {
      pages.push(i);
    }

    return pages;
  });

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const pageSearch = (p: number) => ({ page: p }) as any;

  return (
    <div id="pagination" class="select-none pt-8">
      <div class="flex items-center justify-between gap-2 w-full">
        {/* 上一页 */}
        <div class="flip">
          <Show when={currentPage() > 1}>
            <Link
              to="."
              search={pageSearch(currentPage() - 1)}
              onClick={scrollToTop}
            >
              <svg
                width="22"
                height="22"
                viewBox="0 0 48 48"
                fill="none"
                xmlns="http://www.w3.org/2000/svg"
              >
                <path
                  d="M24 36L12 24L24 12"
                  stroke="currentColor"
                  stroke-width="4"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
                <path
                  d="M36 36L24 24L36 12"
                  stroke="currentColor"
                  stroke-width="4"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
            </Link>
          </Show>
        </div>
        {/* 页码 */}
        <div class="hidden md:flex flex-1 justify-center w-full">
          <For each={renderPageLinks()}>
            {(p) => (
              <Link
                to="."
                search={pageSearch(p)}
                onClick={scrollToTop}
                classList={{ current: p === currentPage() }}
                class="rounded hover:bg-gray-100 dark:hover:bg-zinc-700"
              >
                {p}
              </Link>
            )}
          </For>
        </div>

        <div class="flex items-center justify-center md:hidden">
          {currentPage()} / {totalPages()}
        </div>
        {/* 下一页 */}
        <div class="flip">
          <Show when={currentPage() !== totalPages()}>
            <Link
              to="."
              search={pageSearch(currentPage() + 1)}
              onClick={scrollToTop}
            >
              <svg
                width="22"
                height="22"
                viewBox="0 0 48 48"
                fill="none"
                xmlns="http://www.w3.org/2000/svg"
              >
                <path
                  d="M12 12L24 24L12 36"
                  stroke="currentColor"
                  stroke-width="4"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
                <path
                  d="M24 12L36 24L24 36"
                  stroke="currentColor"
                  stroke-width="4"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
            </Link>
          </Show>
        </div>
      </div>
    </div>
  );
}
