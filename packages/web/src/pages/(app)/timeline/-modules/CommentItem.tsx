import dayjs from "dayjs"
import type { ContentListItem } from "~/api"

interface CommentItemProps {
  comment: ContentListItem
}

export default function CommentItem(props: CommentItemProps) {
  console.log(props.comment);
  return (
    <div class="rounded py-4 space-y-1">
      <div class="flex items-center gap-2 text-sm">
        <span class="font-medium">{props.comment.author_name}</span>
        <span class="opacity-60">
          {dayjs(props.comment.created_at).format("YYYY-MM-DD HH:mm")}
        </span>
      </div>
      <div class="text-sm leading-relaxed">{props.comment.body}</div>
    </div>
  )
}
