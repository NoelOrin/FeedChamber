'use client';

import { type PublicTrustLevel, trustService } from '@feedchamber/api-client';
import { useQuery } from '@tanstack/react-query';

export const trustLevelsQueryKey = ['trust', 'levels', 'public'] as const;

/**
 * 等级名称从后端配置读取（`/levels` 是公开配置接口）。
 * 多实例下名称改动最多 5 分钟随 staleTime 生效，前端不硬编码中文标签。
 */
export function useTrustLevels(): PublicTrustLevel[] {
	const { data } = useQuery({
		queryKey: trustLevelsQueryKey,
		queryFn: () => trustService.getPublicLevels(),
		staleTime: 5 * 60 * 1000,
	});
	return data ?? [];
}

export function trustLevelName(levels: PublicTrustLevel[], level: number | string | null | undefined): string {
	if (level === null || level === undefined || level === '') return '';
	const numeric = Number(level);
	const found = levels.find((l) => l.level === numeric);
	return found?.name || `TL${level}`;
}
