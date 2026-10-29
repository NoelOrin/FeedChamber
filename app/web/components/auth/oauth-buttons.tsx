'use client';

import { type OAuthProviderConfig, oauthService } from '@orinforum/api-client';
import { useQuery } from '@tanstack/react-query';

import { Button } from '~/components/ui/button';
import { CircleDot, Github, Mail, MessageCircle, Send } from '~/lib/icons';

const PROVIDER_META: Record<string, { label: string; icon: typeof CircleDot }> = {
	github: { label: 'GitHub', icon: Github },
	google: { label: 'Google', icon: Mail },
	qq: { label: 'QQ', icon: CircleDot },
	wechat: { label: '微信', icon: MessageCircle },
	weibo: { label: '微博', icon: Send },
};

function enabledProviders(all: Record<string, OAuthProviderConfig> | undefined) {
	if (!all) return [];
	return Object.entries(all)
		.filter(([, config]) => config.enabled)
		.map(([key, config]) => ({
			key,
			authUrl: config.auth_url,
			label: PROVIDER_META[key]?.label ?? key,
			icon: PROVIDER_META[key]?.icon ?? CircleDot,
		}));
}

export function OAuthButtons() {
	const { data } = useQuery({
		queryKey: ['oauth', 'providers'],
		queryFn: () => oauthService.getAllProviders().catch(() => ({}) as Record<string, OAuthProviderConfig>),
	});
	const providers = enabledProviders(data);

	if (!providers.length) return null;

	const handleLogin = (provider: string, authUrl: string) => {
		window.location.assign(authUrl || `/oauth/${provider}/auth`);
	};

	return (
		<div className="mt-5 flex flex-col gap-2">
			<div className="flex items-center gap-3">
				<span aria-hidden="true" className="h-px flex-1 bg-border" />
				<span className="text-[11px] text-muted-foreground">或使用第三方登录</span>
				<span aria-hidden="true" className="h-px flex-1 bg-border" />
			</div>
			{providers.map(({ key, label, icon: Icon, authUrl }) => (
				<Button
					key={key}
					variant="outline"
					size="md"
					className="w-full font-normal"
					onClick={() => handleLogin(key, authUrl)}
				>
					<Icon size={16} aria-hidden="true" />
					<span>{label} 登录</span>
				</Button>
			))}
		</div>
	);
}
