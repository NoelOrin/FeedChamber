import dayjs from "dayjs";
import type { ContentListItem } from "~/api";

interface ContentItemProps {
  content: ContentListItem;
  class?: string;
}

export default function ContentItem(props: ContentItemProps) {
  // console.log(props.content);
  return (
    <>
      <div class={`rounded flex gap-2 min-h-36 ${props.class}`}>
        {/* 左侧头像 */}
        <div class="">
          <img
            src={props.content.author_avatar}
            class="w-12 h-12 rounded-full sticky top-14 shrink-0"
          />
        </div>
        {/* 右侧内容 */}
        <div class="flex-1 space-y-1 text-sm flex flex-col justify-between">
          <div class="flex-1">
            <div class="flex items-center gap-2 text-base">
              <span class="font-bold">{props.content.author_name}</span>
              <span class="opacity-60">
                {dayjs(props.content.created_at).format("YYYY-MM-DD HH:mm")}
              </span>
            </div>
            <div class="leading-relaxed">{props.content.body}</div>
          </div>
        </div>
      </div>
      <div class="divider"></div>
    </>
  );
}
