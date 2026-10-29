import { AuthShell } from '~/components/auth/auth-shell';
import { OAuthButtons } from '~/components/auth/oauth-buttons';
import { OAuthCallback } from '~/components/auth/oauth-callback';
import { Globe } from '~/lib/icons';
import { loadPublicSettings } from '~/lib/queries/server';

export default async function OAuthPage({
	searchParams,
}: {
	searchParams: Promise<{ code?: string | string[]; state?: string | string[]; provider?: string | string[] }>;
}) {
	const settings = await loadPublicSettings();
	const siteName = settings.site_name || 'OrinForum';
	const params = await searchParams;
	const code = typeof params?.code === 'string' ? params.code : '';
	const state = typeof params?.state === 'string' ? params.state : '';
	const provider = state.split(':')[0] || (typeof params?.provider === 'string' ? params.provider : '');

	if (code && provider) {
		return (
			<AuthShell siteName={siteName} title="第三方登录" subtitle={`${provider} 授权回调`}>
				<OAuthCallback provider={provider} code={code} state={state} />
			</AuthShell>
		);
	}

	return (
		<AuthShell
			siteName={siteName}
			title="第三方登录"
			subtitle="选择一个平台登录"
			icon={<Globe size={20} className="text-foreground" aria-hidden="true" />}
		>
			<OAuthButtons />
		</AuthShell>
	);
}
