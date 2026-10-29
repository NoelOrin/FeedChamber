'use client';

import { useState } from 'react';

import { AuthError, AuthField, authSubmitClass } from '~/components/auth/auth-form-shell';
import { Button } from '~/components/ui/button';
import { authClient } from '~/lib/auth';
import { Hash, Lock, LockKeyhole, Mail, User } from '~/lib/icons';
import { toast } from '~/lib/toast';

export function RegisterForm({ registerMode = 'all' }: { registerMode?: string }) {
	const [username, setUsername] = useState('');
	const [email, setEmail] = useState('');
	const [password, setPassword] = useState('');
	const [confirmPwd, setConfirmPwd] = useState('');
	const [inviteCode, setInviteCode] = useState('');
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState('');

	const showInvite = registerMode === 'invite' || registerMode === 'all' || registerMode === 'approve';
	const inviteRequired = registerMode === 'invite';

	async function onSubmit(event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault();
		setError('');
		if (password !== confirmPwd) {
			setError('两次密码不一致');
			return;
		}
		if (password.length < 6) {
			setError('密码至少 6 位');
			return;
		}
		setLoading(true);
		try {
			await authClient.register({
				username: username.trim(),
				email: email.trim(),
				password,
				invite_code: inviteCode.trim() || undefined,
			});
			toast.success(registerMode === 'approve' ? '注册成功，请等待管理员审批' : '注册成功');
			window.location.assign('/timeline');
		} catch (err) {
			setError((err as Error)?.message || '注册失败');
			setLoading(false);
		}
	}

	return (
		<form onSubmit={onSubmit} className="flex flex-col gap-4">
			<AuthError message={error} />

			<AuthField
				id="reg-username"
				label="用户名"
				icon={User}
				type="text"
				value={username}
				onChange={(event) => setUsername(event.currentTarget.value)}
				placeholder="3-32 位字母、数字或下划线"
				autoComplete="username"
				required
			/>
			<AuthField
				id="reg-email"
				label="邮箱"
				icon={Mail}
				type="email"
				value={email}
				onChange={(event) => setEmail(event.currentTarget.value)}
				placeholder="example@mail.com"
				autoComplete="email"
			/>

			{showInvite ? (
				<AuthField
					id="reg-invite"
					label={`邀请码${inviteRequired ? '（必填）' : '（可选）'}`}
					icon={Hash}
					type="text"
					value={inviteCode}
					onChange={(event) => setInviteCode(event.currentTarget.value)}
					placeholder={inviteRequired ? '请输入邀请码' : '如有邀请码可填写'}
					autoComplete="off"
					required={inviteRequired}
				/>
			) : null}

			<AuthField
				id="reg-password"
				label="密码"
				icon={Lock}
				type="password"
				value={password}
				onChange={(event) => setPassword(event.currentTarget.value)}
				placeholder="至少 6 位"
				autoComplete="new-password"
				required
			/>
			<AuthField
				id="reg-confirm"
				label="确认密码"
				icon={LockKeyhole}
				type="password"
				value={confirmPwd}
				onChange={(event) => setConfirmPwd(event.currentTarget.value)}
				placeholder="再次输入密码"
				autoComplete="new-password"
				required
			/>

			<Button type="submit" variant="primary" loading={loading} className={authSubmitClass}>
				注册
			</Button>
		</form>
	);
}
