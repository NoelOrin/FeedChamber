import { createSignal, onMount, Show } from 'solid-js'
import { Copy, Check } from 'lucide-solid'

interface CodeBlockProps {
  code: string
  language?: string
  filename?: string
}

export default function CodeBlock(props: CodeBlockProps) {
  const [copied, setCopied] = createSignal(false)
  let codeRef: HTMLPreElement | undefined

  onMount(() => {
    if (codeRef && typeof window !== 'undefined') {
      import('prismjs').then((Prism) => {
        if (codeRef) {
          Prism.default.highlightElement(codeRef)
        }
      })
    }
  })

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(props.code)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch (err) {
      console.error('Failed to copy:', err)
    }
  }

  return (
    <div class="relative group my-4">
      <Show when={props.filename}>
        <div class="absolute top-0 left-0 px-3 py-1 text-xs text-gray-500 bg-gray-100 dark:bg-gray-800 rounded-bl rounded-tr">
          {props.filename}
        </div>
      </Show>
      <button
        onClick={handleCopy}
        class="absolute top-2 right-2 p-1.5 rounded-md bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 opacity-0 group-hover:opacity-100 transition-opacity"
        title={copied() ? '已复制' : '复制'}
      >
        <Show when={!copied()} fallback={<Check size={16} />}>
          <Copy size={16} />
        </Show>
      </button>
      <pre class="m-0 p-4 overflow-x-auto rounded-lg bg-gray-50 dark:bg-gray-900">
        <code ref={codeRef} class={`language-${props.language || 'text'}`}>
          {props.code}
        </code>
      </pre>
    </div>
  )
}
