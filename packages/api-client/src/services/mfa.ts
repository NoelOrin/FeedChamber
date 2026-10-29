import { post } from '../client';

export const mfaService = {
	setupTotp() {
		return post<{ secret: string; otpauth_url: string }>('/auth/mfa/totp/setup');
	},
	enableTotp(code: string) {
		return post<{ backup_codes: string[] }>('/auth/mfa/totp/enable', { code });
	},
	disableTotp(code: string) {
		return post<null>('/auth/mfa/totp/disable', { code });
	},
	regenerateBackup(code: string) {
		return post<{ backup_codes: string[] }>('/auth/mfa/backup/regenerate', { code });
	},
	verifyLogin(body: { mfa_token: string; code: string }) {
		return post<{ access_token: string }>('/auth/mfa/verify', body);
	},
};
