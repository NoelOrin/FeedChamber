import { createMemo, For } from 'solid-js'
import remarkParse from 'remark-parse'
import remarkGfm from 'remark-gfm'
import { unified } from 'unified'

interface MarkdownRendererProps {
  mdText: string
}

interface MdNode {
  type: string
  value?: string
  children?: MdNode[]
  depth?: number
  ordered?: boolean
  url?: string
  alt?: string
  title?: string
  lang?: string
  checked?: boolean | null
}

function parseMarkdown(md: string): MdNode[] {
  const processor = unified().use(remarkParse).use(remarkGfm)
  const ast = processor.parse(md)
  return (ast as any).children as MdNode[]
}

function renderInline(nodes: MdNode[]): any {
  if (!nodes) return null
  return (
    <For each={nodes}>
      {(node) => {
        if (node.type === 'text') return <>{node.value}</>
        if (node.type === 'strong') return <strong>{renderInline(node.children || [])}</strong>
        if (node.type === 'em') return <em>{renderInline(node.children || [])}</em>
        if (node.type === 'inlineCode') return <code class="bg-gray-100 dark:bg-gray-800 px-1 rounded text-sm">{node.value}</code>
        if (node.type === 'link') return <a href={node.url || '#'} class="text-blue-500 hover:underline">{renderInline(node.children || [])}</a>
        if (node.type === 'image') return <img src={node.url || ''} alt={node.alt || ''} class="max-w-full" />
        if (node.type === 'delete') return <del>{renderInline(node.children || [])}</del>
        if (node.type === 'break') return <br />
        return <>{node.value || ''}</>
      }}
    </For>
  )
}

function renderNode(node: MdNode): any {
  switch (node.type) {
    case 'heading': {
      const depth = node.depth || 1
      const content = renderInline(node.children || [])
      if (depth === 1) return <h1 class="font-bold text-3xl my-4">{content}</h1>
      if (depth === 2) return <h2 class="font-bold text-2xl my-4">{content}</h2>
      if (depth === 3) return <h3 class="font-bold text-xl my-4">{content}</h3>
      if (depth === 4) return <h4 class="font-bold text-lg my-4">{content}</h4>
      if (depth === 5) return <h5 class="font-bold text-base my-4">{content}</h5>
      return <h6 class="font-bold text-sm my-4">{content}</h6>
    }
    case 'paragraph':
      return <p class="my-3 leading-relaxed">{renderInline(node.children || [])}</p>
    case 'list':
      if (node.ordered) {
        return (
          <ol class="list-decimal pl-6 my-3 space-y-1">
            <For each={node.children || []}>
              {(item) => <li>{renderInline(item.children || [])}</li>}
            </For>
          </ol>
        )
      }
      return (
        <ul class="list-disc pl-6 my-3 space-y-1">
          <For each={node.children || []}>
            {(item) => <li>{renderInline(item.children || [])}</li>}
          </For>
        </ul>
      )
    case 'listItem':
      return <li>{renderInline(node.children || [])}</li>
    case 'blockquote':
      return (
        <blockquote class="border-l-4 border-gray-300 dark:border-gray-600 pl-4 my-3 italic">
          {renderInline(node.children || [])}
        </blockquote>
      )
    case 'code':
      return (
        <pre class="bg-gray-50 dark:bg-gray-900 p-4 rounded-lg overflow-x-auto my-3">
          <code class={`language-${node.lang || 'text'}`}>{node.value}</code>
        </pre>
      )
    case 'html':
      return <div innerHTML={node.value || ''} />
    case 'thematicBreak':
      return <hr class="my-6 border-gray-300 dark:border-gray-600" />
    case 'link':
      return <a href={node.url || '#'} class="text-blue-500 hover:underline">{renderInline(node.children || [])}</a>
    case 'image':
      return <img src={node.url || ''} alt={node.alt || ''} class="max-w-full my-3" />
    default:
      return <div>{renderInline(node.children || [])}</div>
  }
}

export default function MarkdownRenderer(props: MarkdownRendererProps) {
  const ast = createMemo<MdNode[]>(() => {
    if (!props.mdText) return []
    try {
      return parseMarkdown(props.mdText)
    } catch (e) {
      console.error('Markdown parse error:', e)
      return []
    }
  })

  return (
    <div class="prose dark:prose-invert max-w-none">
      <For each={ast()}>
        {(node) => renderNode(node)}
      </For>
    </div>
  )
}
