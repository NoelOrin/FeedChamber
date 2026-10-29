export function formatSmartDate(dateString: string, now?: Date): string {
	const date = new Date(dateString);
	const currentTime = now ?? new Date();
	const diffMs = currentTime.getTime() - date.getTime();
	const diffSec = Math.floor(diffMs / 1000);
	const diffMin = Math.floor(diffSec / 60);
	const diffHour = Math.floor(diffMin / 60);
	const diffDay = Math.floor(diffHour / 24);
	const diffWeek = Math.floor(diffDay / 7);
	const diffMonth = Math.floor(diffDay / 30);

	if (diffSec < 60) return '刚刚';
	if (diffMin < 60) return `${diffMin} 分钟前`;
	if (diffHour < 24) return `${diffHour} 小时前`;
	if (diffDay < 7) return `${diffDay} 天前`;
	if (diffWeek < 4) return `${diffWeek} 周前`;
	if (diffMonth < 12) return `${diffMonth} 个月前`;

	const year = date.getFullYear();
	const month = String(date.getMonth() + 1).padStart(2, '0');
	const day = String(date.getDate()).padStart(2, '0');
	return `${year}年${month}月${day}日`;
}
