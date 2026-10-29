'use client';

import { useRouter } from 'next/navigation';

import { Button } from '~/components/ui/button';

interface Props {
	/** 该页面要求的权限码（anyOf），用于提示与排查。 */
	required?: readonly string[];
}

/**
 * 模块级 403：已进后台、但没有该模块权限时，在壳层内替换页面内容。
 * 菜单一侧已按权限过滤，正常点不到；手工敲 URL 或权限刚被回收时会走到这里。
 */
export function ModuleForbidden({ required = [] }: Props) {
	const router = useRouter();

	return (
		<div className="flex min-h-[60vh] flex-col items-center justify-center gap-4 text-center">
			<div className="flex flex-col gap-1.5">
				<span className="text-muted-foreground text-[11px] font-medium tracking-[0.14em] uppercase">403</span>
				<h1 className="text-xl font-semibold tracking-tight text-foreground">无权限访问该模块</h1>
				<p className="text-muted-foreground max-w-[440px] text-[13px]">
					你的账号没有打开这个页面的权限。菜单里已隐藏该入口；如果这是工作所需，请联系管理员调整角色权限。
				</p>
			</div>
			<div className="flex items-center gap-2">
				<Button size="sm" onClick={() => router.push('/admin')}>
					返回运营概览
				</Button>
				<Button variant="outline" size="sm" onClick={() => router.back()}>
					返回上一页
				</Button>
			</div>
			{required.length > 0 ? (
				<p className="text-muted-foreground text-xs">
					需要权限：
					<code className="bg-muted rounded px-1.5 py-0.5 text-foreground">{required.join(' 或 ')}</code>
				</p>
			) : null}
		</div>
	);
}
