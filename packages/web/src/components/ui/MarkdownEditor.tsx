import { createSignal, Show } from 'solid-js'
import { Bold, Italic, Code, Link, List, ListOrdered, Quote, Eye, EyeOff } from 'lucide-solid'
import MarkdownRenderer from '../content/MarkdownRenderer'

interface MarkdownEditorProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
  minRows?: number
  disabled?: boolean
  error?: string
}

export default function MarkdownEditor(props: MarkdownEditorProps) {
  const [showPreview, setShowPreview] = createSignal(false)
  let textareaRef: HTMLTextAreaElement | undefined

  const insertMarkdown = (before: string, after: string = '') => {
    const textarea = textareaRef
    if (!textarea) return

    const start = textarea.selectionStart
    const end = textarea.selectionEnd
    const text = props.value
    const selected = text.substring(start, end)

    const newText = text.substring(0, start) + before + selected + after + text.substring(end)
    props.onChange(newText)

    // Restore cursor position
    requestAnimationFrame(() => {
      textarea.focus()
      const newPos = start + before.length + (selected ? selected.length : 0)
      textarea.setSelectionRange(newPos, newPos)
    })
  }

  const toolbarItems = [
    { icon: Bold, action: () => insertMarkdown('**', '**'), title: '粗体' },
    { icon: Italic, action: () => insertMarkdown('*', '*'), title: '斜体' },
    { icon: Code, action: () => insertMarkdown('`', '`'), title: '行内代码' },
    { icon: Link, action: () => insertMarkdown('[', '](url)'), title: '链接' },
    { icon: List, action: () => insertMarkdown('\n- '), title: '无序列表' },
    { icon: ListOrdered, action: () => insertMarkdown('\n1. '), title: '有序列表' },
    { icon: Quote, action: () => insertMarkdown('\n> '), title: '引用' },
  ]

  return (
    <div class="space-y-2">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-1">
          {toolbarItems.map((item) => (
            <button
              type="button"
              onClick={item.action}
              disabled={props.disabled || showPreview()}
              class="p-1.5 rounded hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
              title={item.title}
            >
              <item.icon size={16} />
            </button>
          ))}
        </div>
        <button
          type="button"
          onClick={() => setShowPreview(!showPreview())}
          class="flex items-center gap-1 px-2 py-1 text-xs rounded hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
          title={showPreview() ? '编辑' : '预览'}
        >
          <Show when={showPreview()} fallback={<Eye size={14} />}>
            <EyeOff size={14} />
          </Show>
          <span>{showPreview() ? '编辑' : '预览'}</span>
        </button>
      </div>

      <Show
        when={!showPreview()}
        fallback={
          <div class="min-h-[80px] p-3 border border-border rounded-md bg-gray-50 dark:bg-gray-900">
            {props.value ? (
              <MarkdownRenderer mdText={props.value} />
            ) : (
              <p class="text-sm text-remark opacity-60">预览为空</p>
            )}
          </div>
        }
      >
        <textarea
          ref={textareaRef}
          value={props.value}
          onInput={(e) => props.onChange(e.currentTarget.value)}
          placeholder={props.placeholder}
          rows={props.minRows ?? 3}
          disabled={props.disabled}
          class="w-full resize-none rounded-md border border-border bg-transparent px-3 py-2 text-sm outline-none focus:border-primary transition-colors disabled:opacity-50"
        />
      </Show>

      <Show when={props.error}>
        <p class="text-xs text-red-500">{props.error}</p>
      </Show>
    </div>
  )
}
