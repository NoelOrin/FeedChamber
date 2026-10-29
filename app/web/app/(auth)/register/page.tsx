import Link from 'next/link';

import { AuthSplitShell } from '~/components/auth/auth-split-shell';
import { RegisterForm } from '~/components/auth/register-form';
import { loadPublicSettings } from '~/lib/queries/server';

export default async function RegisterPage() {
	const settings = await loadPublicSettings();
	const siteName = settings.site_name || 'Feed Chamber';

	return (
		<AuthSplitShell
			siteName={siteName}
			siteDescription={settings.site_description}
			siteLogo={settings.site_logo}
			title="创建账号"
			subtitle="加入社区，开始你的第一次发言。"
			footer={
				<>
					已有账号？
					<Link href="/login" className="ml-1 font-medium text-brand-text hover:underline">
						去登录
					</Link>
				</>
			}
		>
			<RegisterForm registerMode={settings.register_mode || 'all'} />
		</AuthSplitShell>
	);
}
