'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '~/components/ui/form';
import { Input } from '~/components/ui/input';
import { authClient } from '~/lib/auth';
import { Lock } from '~/lib/icons';
import { useSiteName } from '~/lib/settings';
import { toast } from '~/lib/toast';

const schema = z
	.object({
		new_password: z.string().min(8, '密码至少 8 位！'),
		confirm: z.string().min(1, '请再次输入新密码！'),
	})
	.refine((v) => v.new_password === v.confirm, {
		message: '两次输入的密码不一致！',
		path: ['confirm'],
	});

type Values = z.infer<typeof schema>;

export default function FirstChangePasswordPage() {
	const siteName = useSiteName();
	const [loading, setLoading] = useState(false);

	const form = useForm<Values>({
		resolver: zodResolver(schema),
		defaultValues: { new_password: '', confirm: '' },
	});

	async function onSubmit(values: Values) {
		setLoading(true);
		try {
			await authClient.firstChangePassword({ new_password: values.new_password });
			toast.success('密码修改成功，请重新登录');
			// 改密后旧会话仍带 must-change 标记，登出再用新密码登录一次最干净。
			await authClient.logout();
			window.location.assign('/login?redirect=%2Fadmin');
		} catch (error: any) {
			toast.error(error?.response?.data?.msg || error?.message || '修改失败，请稍后重试！');
		} finally {
			setLoading(false);
		}
	}

	return (
		<div className="flex min-h-screen items-center justify-center bg-[image:var(--dashboard-bg-gradient)]">
			<div className="w-full max-w-[420px] px-4">
				<Card className="px-6 py-8">
					<div className="mb-8 text-center">
						<div className="bg-accent mb-4 inline-flex size-16 items-center justify-center rounded-full">
							<Lock className="text-warning size-6" />
						</div>
						<h1 className="mb-2 text-2xl font-bold text-foreground">{siteName} 后台</h1>
						<p className="text-muted-foreground m-0">首次登录，请修改默认密码后继续</p>
					</div>

					<Form {...form}>
						<form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4">
							<FormField
								control={form.control}
								name="new_password"
								render={({ field }) => (
									<FormItem>
										<FormLabel>新密码</FormLabel>
										<FormControl>
											<Input type="password" placeholder="新密码" autoComplete="new-password" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="confirm"
								render={({ field }) => (
									<FormItem>
										<FormLabel>确认新密码</FormLabel>
										<FormControl>
											<Input type="password" placeholder="确认新密码" autoComplete="new-password" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<Button type="submit" size="lg" className="w-full" loading={loading}>
								修改密码
							</Button>
						</form>
					</Form>
				</Card>
			</div>
		</div>
	);
}
