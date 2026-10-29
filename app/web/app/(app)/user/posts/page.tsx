import { redirect } from 'next/navigation';

/**
 * 旧 `/(app)/user/posts.tsx` 的 beforeLoad 语义：`/user/posts` 只是 `/user` 的历史别名，
 * e2e 锁死这条 URL，因此保持重定向而不是另建一份列表页。
 * App Router 里静态段 `posts/` 优先于同级 `[userId]/`，不会被动态段吞掉。
 */
export default function UserPostsPage() {
	redirect('/user');
}
