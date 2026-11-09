import { post } from '../client';
import type { MailTestReport } from '../types';

export const settingService = {
	getPublic() {
		return post<Record<string, string>>('/settings/public');
	},

	getAll() {
		return post<Record<string, string>>('/admin/settings');
	},

	get(key: string) {
		return post<string>(`/admin/settings/${key}`);
	},

	update(key: string, value: string) {
		return post<null>(`/admin/settings/${key}/update`, { value });
	},

	batchUpdate(settings: Record<string, string>) {
		return post<null>('/admin/settings/batch', { settings });
	},

	delete(key: string) {
		return post<null>(`/admin/settings/${key}/delete`);
	},

	/** 用当前已保存的邮件配置发一封测试邮件；投递失败回 ok=false 而不是抛错。 */
	sendTestEmail(to: string) {
		return post<MailTestReport>('/admin/settings/email/test', { to });
	},
};
