import { PostComposer } from '~/components/post/composer';

/** URL `/spaces/{slug}/post`（版内发帖，AGENTS.md 路由表锁定）。表单与 /timeline/create 共用 PostComposer。 */
export default async function SpacePostPage({ params }: { params: Promise<{ slug: string }> }) {
	const { slug } = await params;
	return <PostComposer spaceSlug={slug} />;
}
