'use client';

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { useLocale } from '~/lib/i18n';
import { toast } from '~/lib/toast';

export function DashboardLanguage() {
	const { locale: language, setLocale: changeLanguage } = useLocale();

	function handleLanguageChange(value: string) {
		changeLanguage(value);
		toast.success('语言设置已保存');
	}

	return (
		<Select value={language} onValueChange={handleLanguageChange}>
			<SelectTrigger className="max-w-[400px]">
				<SelectValue placeholder="选择语言" />
			</SelectTrigger>
			<SelectContent>
				<SelectItem value="zh-CN">中文</SelectItem>
				<SelectItem value="en-US">English</SelectItem>
			</SelectContent>
		</Select>
	);
}
