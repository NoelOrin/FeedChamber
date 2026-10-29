import { post } from '../client';

export type Theme = {
	id: string;
	name: string;
	dark: boolean;
};

export const themeService = {
	list() {
		return post<Theme[]>('/themes/list', {});
	},
	getPreference() {
		return post<{ theme: string }>('/themes/preference', {});
	},
	setPreference(theme: string) {
		return post<null>('/themes/preference/update', { theme });
	},
};
