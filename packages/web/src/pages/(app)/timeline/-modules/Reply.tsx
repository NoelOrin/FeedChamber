import { createSignal } from "solid-js";
import { contentService } from "~/api";
import type { ContentListItem } from "~/api";
import MarkdownEditor from "~/components/ui/MarkdownEditor";

interface ReplyProps {
  postId: string;
  rootContentId: string;
  onSuccess?: (content: ContentListItem) => void;
}

export default function Reply(props: ReplyProps) {
  const [body, setBody] = createSignal("");
  const [submitting, setSubmitting] = createSignal(false);
  const [error, setError] = createSignal("");

  const isLoggedIn = () =>
    typeof localStorage !== "undefined" && !!localStorage.getItem("access_token");

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    const text = body().trim();
    if (!text) return;
    setSubmitting(true);
    setError("");
    try {
      const created = await contentService.create(props.postId, {
        body: text,
        parentId: props.rootContentId,
      });
      setBody("");
      props.onSuccess?.({
        id: created.id,
        author_id: created.author_id,
        author_name: "",
        author_avatar: "",
        body: created.body,
        root_id: created.root_id,
        parent_id: created.parent_id,
        like_count: created.like_count,
        created_at: created.created_at,
        updated_at: created.updated_at,
      });
    } catch (err: any) {
      setError(err?.message ?? "发布失败，请重试");
    } finally {
      setSubmitting(false);
    }
  };

  if (!isLoggedIn()) {
    return (
      <div class="card p-4 text-sm text-remark text-center">
        <a href="/login" class="text-primary hover:underline">登录</a>后即可参与讨论
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit} class="card p-4 space-y-3">
      <MarkdownEditor
        value={body()}
        onChange={setBody}
        placeholder="写下你的回复...支持 Markdown 语法"
        minRows={3}
        disabled={submitting()}
        error={error()}
      />
      <div class="flex justify-end">
        <button
          type="submit"
          disabled={submitting() || !body().trim()}
          class="btn px-4 py-1.5 text-sm rounded-md disabled:opacity-40"
        >
          {submitting() ? "发布中..." : "发布回复"}
        </button>
      </div>
    </form>
  );
}
