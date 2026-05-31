import MarkdownRenderer from "~/components/content/MarkdownRenderer"

interface PostContentProps {
  body: string
}

export default function PostContent(props: PostContentProps) {
  return (
    // <div class="border-b pb-6">
      <MarkdownRenderer mdText={props.body} />
    // </div>
  )
}
