import { createSignal, onMount, For } from 'solid-js'
import { Link } from '@tanstack/solid-router'
import BackToTop from './BackToTop'

interface PaginatorProps {
  pageNum: number
  totalPages: number
}

export default function Paginator(props: PaginatorProps) {
  const [currentPage, setCurrentPage] = createSignal(1)

  onMount(() => {
    const urlParams = new URLSearchParams(window.location.search)
    const page = parseInt(urlParams.get('page') || '1', 10)
    setCurrentPage(page)
  })

  const scrollToTop = () => {
    BackToTop
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  const renderPageLinks = () => {
    const pages: number[] = []
    const current = currentPage()
    const total = props.totalPages

    if (current - 2 > 0) pages.push(current - 2)
    if (current - 1 > 0) pages.push(current - 1)
    pages.push(current)
    if (current + 1 <= total) pages.push(current + 1)
    if (current + 2 <= total) pages.push(current + 2)

    return pages
  }

  return (
    <div id="pagination" class="select-none pt-8">
      <div class="flex items-center justify-center gap-2">
        {currentPage() !== 1 ? (
          <div class="flip">
            <Link
              to="/"
              search={{ page: currentPage() - 1 }}
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
          </div>
        ) : (
          <div class="invisible flip">
            <a href=""></a>
          </div>
        )}

        <div class="hidden md:flex gap-2">
          <For each={renderPageLinks()}>
            {(page) => (
              <Link
                to="/"
                search={{ page }}
                onClick={scrollToTop}
                classList={{ current: page === currentPage() }}
                class="px-3 py-1 rounded hover:bg-gray-100 dark:hover:bg-zinc-700"
              >
                {page}
              </Link>
            )}
          </For>
        </div>

        <div class="flex items-center justify-center md:hidden">
          {currentPage()} / {props.totalPages}
        </div>

        {currentPage() !== props.totalPages ? (
          <div class="flip">
            <Link
              to="/"
              search={{ page: currentPage() + 1 }}
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
          </div>
        ) : (
          <div class="invisible flip">
            <a href=""></a>
          </div>
        )}
      </div>
    </div>
  )
}
