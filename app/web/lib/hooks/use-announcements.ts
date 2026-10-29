'use client';

import { type Announcement, announcementService } from '@orinforum/api-client';
import { useQuery } from '@tanstack/react-query';
import { useCallback } from 'react';

import { qk } from '~/lib/query/keys';

/* 稳定的空数组：避免每次渲染都新建引用，下游 useMemo/effect 才会被无谓地打断 */
const NO_ANNOUNCEMENTS: Announcement[] = [];

interface AnnouncementsResult {
	announcements: Announcement[];
	loading: boolean;
	error: string | null;
	refresh: () => void;
}

/** 已发布公告（旧 Solid hooks/useAnnouncements.ts 平移，改为 TanStack Query 驱动）。 */
export function useAnnouncements(): AnnouncementsResult {
	const { data, isPending, error, refetch } = useQuery({
		queryKey: qk.announcements.published,
		queryFn: () => announcementService.listPublished(),
	});

	const refresh = useCallback(() => {
		void refetch();
	}, [refetch]);

	return {
		announcements: data?.items ?? NO_ANNOUNCEMENTS,
		loading: isPending,
		error: error ? (error instanceof Error ? error.message : 'Failed to load announcements') : null,
		refresh,
	};
}
