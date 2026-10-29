import { PostComposer } from '~/components/post/composer';
import { requireAuth } from '~/lib/auth/guards';

/**
 * URL `/timeline/create` —— Solid 版 pages/(app)/timeline/create.tsx 的路由壳，表单在共用 PostComposer。
 *
 * 发帖是结构性私有面：(app)/layout 不再兜底 requireAuth（公开内容面共用该布局），
 * 因此本页自己守。见 proxy.ts 的 PROTECTED 注释。
 */
export default async function CreatePostPage() {
	await requireAuth('/timeline/create');
	return <PostComposer />;
}
