'use client';

import { type MailTestReport, settingService } from '@feedchamber/api-client';
import { useState } from 'react';

import { Button } from '~/components/ui/button';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { toast } from '~/lib/toast';

/*
 * 邮件配置自测面板。
 *
 * 为什么需要它：配置错误的唯一暴露时机本来是用户点「忘记密码」，而那条路径把
 * 失败原因以 AppError 吐给终端用户——既泄露内部信息，管理员又拿不到诊断。
 *
 * 契约：后端投递失败回 HTTP 200 + report.ok=false，所以这里「不抛错」是正常的，
 * 必须按 report.ok 分流提示，不能靠 catch 判断成败。
 */
export function EmailTestPanel() {
	const [to, setTo] = useState('');
	const [sending, setSending] = useState(false);
	const [report, setReport] = useState<MailTestReport | null>(null);

	async function handleSend() {
		const target = to.trim();
		if (!target) {
			toast.error('请先填写收件人邮箱');
			return;
		}
		setSending(true);
		setReport(null);
		try {
			const result = await settingService.sendTestEmail(target);
			setReport(result);
			if (result.ok) {
				toast.success(result.notice ? `已发送（${result.notice}）` : '测试邮件已发送');
			} else {
				toast.error(result.error || '发送失败');
			}
		} catch (error: any) {
			toast.error(error.message || '发送失败');
		} finally {
			setSending(false);
		}
	}

	return (
		<div className="mt-3 rounded-md border border-border p-3">
			<div className="mb-1 text-[13px] font-medium text-foreground">邮件发送自测</div>
			<p className="mb-2 text-xs text-muted-foreground">
				使用「已保存」的配置真发一封测试邮件。改了上面的配置请先点「保存设置」，否则测的还是旧配置。
			</p>

			<div className="flex flex-wrap items-end gap-2">
				<div className="flex min-w-[240px] flex-1 flex-col gap-1.5">
					<Label htmlFor="email-test-to">收件人</Label>
					<Input id="email-test-to" placeholder="you@example.com" value={to} onChange={(e) => setTo(e.target.value)} />
				</div>
				<Button size="sm" loading={sending} onClick={handleSend}>
					发送测试邮件
				</Button>
			</div>

			{report ? (
				<div
					className={`mt-3 rounded-md border p-2.5 text-xs ${
						report.ok
							? 'border-emerald-500/40 bg-emerald-500/5 text-emerald-700 dark:text-emerald-400'
							: 'border-destructive/40 bg-destructive/5 text-destructive'
					}`}
				>
					<div className="font-medium">{report.ok ? '发送成功' : '发送失败'}</div>
					<div className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-muted-foreground">
						<span>通道：{report.provider || 'log'}</span>
						{report.host ? (
							<span>
								服务器:{report.host}:{report.port}
							</span>
						) : null}
						{report.tls_mode ? (
							<span>{report.tls_mode === 'implicit' ? '隐式 TLS (465)' : 'STARTTLS (587)'}</span>
						) : null}
						{report.from ? <span>发件人:{report.from}</span> : null}
						<span>耗时 {report.elapsed_ms}ms</span>
					</div>
					{report.error ? <div className="mt-1 break-words">原因：{report.error}</div> : null}
					{report.notice ? <div className="mt-1 break-words">注意：{report.notice}</div> : null}
				</div>
			) : null}
		</div>
	);
}
