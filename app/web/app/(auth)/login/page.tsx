import Link from 'next/link';

import { AuthSplitShell } from '~/components/auth/auth-split-shell';
import { LoginForm } from '~/components/auth/login-form';
import { loadPublicSettings } from '~/lib/queries/server';

export default async function LoginPage({ searchParams }: { searchParams: Promise<{ redirect?: string | string[] }> }) {
	const settings = await loadPublicSettings();
	const siteName = settings.site_name || 'Feed Chamber';
	const raw = (await searchParams)?.redirect;
	const redirect = typeof raw === 'string' ? raw : undefined;

	return (
		<AuthSplitShell
			siteName={siteName}
			siteDescription={settings.site_description}
			siteLogo={settings.site_logo}
			title="登录"
			subtitle="欢迎回来，继续上次的讨论。"
			footer={
				<>
					还没有账号？
					<Link href="/register" className="ml-1 font-medium text-brand-text hover:underline">
						立即注册
					</Link>
				</>
			}
		>
			<LoginForm redirect={redirect} />
		</AuthSplitShell>
	);
}
