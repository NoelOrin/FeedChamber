import dayjs from "dayjs"
import type { PostListItem } from "~/api"

interface PostHeaderProps {
  post: PostListItem
}

export default function PostHeader(props: PostHeaderProps) {
  return (
    <div>
      <h1 class="font-bold text-2xl leading-tight">{props.post.title}</h1>
      <div class="mt-2 text-sm flex items-center gap-3">
        <span>{props.post.author_name}</span>
        <span>{props.post.category_name}</span>
        <span>{dayjs(props.post.created_at).format("YYYY-MM-DD HH:mm")}</span>
        <span>{props.post.view_count} 阅读</span>
        <span>{props.post.comment_count} 评论</span>
      </div>
    </div>
  )
}
