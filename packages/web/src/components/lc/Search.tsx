import { createSignal, onMount, onCleanup } from 'solid-js'
import clsx from 'clsx'

interface SearchProps {
  onClose: () => void
}

export default function Search(props: SearchProps) {
  const [query, setQuery] = createSignal('')
  const [results, setResults] = createSignal<Array<{ title: string; url: string; content: string }>>([])
  const [loading, setLoading] = createSignal(false)

  let inputRef: HTMLInputElement | undefined
  let overlayRef: HTMLDivElement | undefined

  const close = () => {
    props.onClose()
  }

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Escape') {
      close()
    }
  }

  const handleOverlayClick = (e: MouseEvent) => {
    if (e.target === overlayRef) {
      close()
    }
  }

  // Basic search function - replace with Algolia client later
  const performSearch = async (q: string) => {
    if (!q.trim()) {
      setResults([])
      return
    }
    setLoading(true)
    try {
      // TODO: Connect to Algolia or other search provider
      // const { hits } = await algoliaClient.search(q)
      // setResults(hits)
      setResults([])
    } catch {
      setResults([])
    } finally {
      setLoading(false)
    }
  }

  let searchTimeout: ReturnType<typeof setTimeout> | null = null

  const handleInput = (value: string) => {
    setQuery(value)
    if (searchTimeout) clearTimeout(searchTimeout)
    searchTimeout = setTimeout(() => performSearch(value), 300)
  }

  onMount(() => {
    inputRef?.focus()
    document.addEventListener('keydown', handleKeyDown)
  })

  onCleanup(() => {
    document.removeEventListener('keydown', handleKeyDown)
    if (searchTimeout) clearTimeout(searchTimeout)
  })

  return (
    <div
      ref={overlayRef}
      class="fixed inset-0 z-[100] flex items-start justify-center bg-black/50 pt-[20vh]"
      onClick={handleOverlayClick}
    >
      <div class="card mx-4 w-full max-w-[600px] p-4">
        <div class="flex items-center gap-2">
          <svg
            class="shrink-0"
            fill="none"
            height="20"
            viewBox="0 0 48 48"
            width="20"
            xmlns="http://www.w3.org/2000/svg"
          >
            <path
              d="M21 38C30.3888 38 38 30.3888 38 21C38 11.6112 30.3888 4 21 4C11.6112 4 4 11.6112 4 21C4 30.3888 11.6112 38 21 38Z"
              fill="none"
              stroke="currentColor"
              stroke-linejoin="round"
              stroke-width="3"
            />
            <path
              d="M26.657 14.3431C25.2093 12.8954 23.2093 12 21.0001 12C18.791 12 16.791 12.8954 15.3433 14.3431"
              stroke="currentColor"
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="3"
            />
            <path
              d="M33.2216 33.2217L41.7069 41.707"
              stroke="currentColor"
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="3"
            />
          </svg>
          <input
            ref={inputRef}
            type="text"
            placeholder="搜索文章..."
            class="w-full bg-transparent text-lg outline-none"
            value={query()}
            onInput={(e) => handleInput(e.currentTarget.value)}
          />
          <button
            class="shrink-0 p-1 duration-200 btn"
            onClick={close}
          >
            <svg
              fill="none"
              height="16"
              viewBox="0 0 48 48"
              width="16"
              xmlns="http://www.w3.org/2000/svg"
            >
              <path
                d="M38 10L10 38M10 10L38 38"
                stroke="currentColor"
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="4"
              />
            </svg>
          </button>
        </div>

        <div class="mt-3 max-h-[50vh] overflow-y-auto">
          {loading() && (
            <div class="py-4 text-center text-sm text-remark">搜索中...</div>
          )}

          {!loading() && query() && results().length === 0 && (
            <div class="py-4 text-center text-sm text-remark">
              未找到相关结果
            </div>
          )}

          {results().map((result) => (
            <a
              href={result.url}
              class="block rounded-lg p-3 duration-200 hover:bg-foreground/5"
              onClick={close}
            >
              <div class="font-bold">{result.title}</div>
              <div class="mt-1 text-sm text-remark line-clamp-2">
                {result.content}
              </div>
            </a>
          ))}
        </div>
      </div>
    </div>
  )
}
