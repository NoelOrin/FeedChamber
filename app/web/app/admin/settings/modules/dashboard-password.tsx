'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { authService } from '@orinforum/api-client';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import { Button } from '~/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '~/components/ui/form';
import { Input } from '~/components/ui/input';
import { Save } from '~/lib/icons';
import { toast } from '~/lib/toast';

/* 「两次密码一致」从 handleChangePassword 里的手工判断升到 schema，
   这样确认框本身就会报错，而不是提交后才弹一条 message。 */
const passwordSchema = z
	.object({
		oldPassword: z.string().min(1, '请输入当前密码'),
		newPassword: z.string().min(6, '密码至少 6 个字符'),
		confirmPassword: z.string().min(1, '请再次输入新密码'),
	})
	.refine((v) => v.newPassword === v.confirmPassword, {
		message: '两次输入的新密码不一致',
		path: ['confirmPassword'],
	});

type PasswordValues = z.infer<typeof passwordSchema>;

export function DashboardPassword() {
	const [passwordLoading, setPasswordLoading] = useState(false);

	const form = useForm<PasswordValues>({
		resolver: zodResolver(passwordSchema),
		defaultValues: { oldPassword: '', newPassword: '', confirmPassword: '' },
	});

	async function handleChangePassword(values: PasswordValues) {
		setPasswordLoading(true);
		try {
			await authService.changePassword({
				old_password: values.oldPassword,
				new_password: values.newPassword,
			});
			toast.success('密码修改成功');
			form.reset();
		} catch (e: any) {
			toast.error(e.message || '密码修改失败');
		} finally {
			setPasswordLoading(false);
		}
	}

	return (
		<Form {...form}>
			<form onSubmit={form.handleSubmit(handleChangePassword)} className="flex max-w-[400px] flex-col gap-3">
				<FormField
					control={form.control}
					name="oldPassword"
					render={({ field }) => (
						<FormItem>
							<FormLabel>当前密码</FormLabel>
							<FormControl>
								<Input type="password" placeholder="请输入当前密码" autoComplete="current-password" {...field} />
							</FormControl>
							<FormMessage />
						</FormItem>
					)}
				/>
				<FormField
					control={form.control}
					name="newPassword"
					render={({ field }) => (
						<FormItem>
							<FormLabel>新密码</FormLabel>
							<FormControl>
								<Input type="password" placeholder="请输入新密码" autoComplete="new-password" {...field} />
							</FormControl>
							<FormMessage />
						</FormItem>
					)}
				/>
				<FormField
					control={form.control}
					name="confirmPassword"
					render={({ field }) => (
						<FormItem>
							<FormLabel>确认新密码</FormLabel>
							<FormControl>
								<Input type="password" placeholder="请再次输入新密码" autoComplete="new-password" {...field} />
							</FormControl>
							<FormMessage />
						</FormItem>
					)}
				/>
				<div>
					<Button type="submit" size="sm" loading={passwordLoading}>
						<Save className="size-3.5" />
						修改密码
					</Button>
				</div>
			</form>
		</Form>
	);
}
