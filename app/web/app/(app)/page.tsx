import { redirect } from 'next/navigation';

/** 旧 (app)/index.tsx 的 beforeLoad 重定向，URL 契约不允许改成 301 之外的形态。 */
export default function IndexPage() {
	redirect('/timeline');
}
