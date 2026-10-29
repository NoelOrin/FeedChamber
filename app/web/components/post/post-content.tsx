'use client';

import type { ContentListItem, PostDetail } from '@orinforum/api-client';

import { ContentItem } from '~/components/content/content-item';
import { MarkdownRenderer } from '~/components/content/markdown-renderer';

interface PostContentProps {
	post: PostDetail;
}

/**
 * 正文卡的内容层（旧 -modules/PostContent.tsx + PostDetailView 里 rootContent() 的合并）。
 *
 * 旧版把主题正文当作第 0 号楼层交给 ContentItem 渲染，因此正文卡自带 §11.4 的「楼层尾」
 * （点赞 · N / 版本 / # 楼层号），这里 1:1 保留；拿不到 content_id 时退化为纯 Markdown。
 * 旧版外层还包了一个 `prose` div，现在 Prose 已在 MarkdownRenderer 内部，不再重复包裹。
 */
export function PostContent({ post }: PostContentProps) {
	const contentId = post.content_id;
	if (!contentId || !post.id) {
		if (!post.body?.trim()) return <div className="text-sm text-muted-foreground">正文加载中…</div>;
		return <MarkdownRenderer mdText={post.body} />;
	}

	const root: ContentListItem = {
		id: contentId,
		root_id: contentId,
		parent_id: '',
		author_id: post.author_id,
		author_name: post.author_name,
		author_avatar: post.author_avatar,
		body: post.body ?? '',
		like_count: post.like_count,
		post_id: post.id,
		post_title: post.title,
		created_at: post.created_at,
		updated_at: post.updated_at,
	};

	return <ContentItem content={root} postId={post.id} mode="root" className="py-0" />;
}

export default PostContent;
