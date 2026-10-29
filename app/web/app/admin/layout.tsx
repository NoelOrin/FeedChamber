import type { Metadata } from 'next';

import './admin.css';

import { AdminShell } from '~/components/admin/admin-shell';

/**
 * `/admin/**` 的入口。这里只做两件服务端能做的事：声明元信息（后台不进搜索引擎）、
 * 引入后台样式；真正的判定与渲染全在客户端（`AdminShell`），所以后台内容不走 SSR。
 */
export const metadata: Metadata = {
	title: '管理后台',
	robots: { index: false, follow: false, nocache: true },
};

export default function AdminLayout({ children }: { children: React.ReactNode }) {
	return <AdminShell>{children}</AdminShell>;
}
