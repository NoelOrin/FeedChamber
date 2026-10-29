'use client';

import type { Announcement } from '@orinforum/api-client';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';

import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Prose } from '~/components/ui/prose';
import { useAnnouncements } from '~/lib/hooks/use-announcements';
import { Info, Megaphone, X } from '~/lib/icons';
import { toast } from '~/lib/toast';

const DISMISS_KEY = 'announcement_dismissed';

function getDismissedIds(): Set<string> {
	if (typeof localStorage === 'undefined') return new Set();
	try {
		const raw = localStorage.getItem(DISMISS_KEY);
		if (!raw) return new Set();
		const obj = JSON.parse(raw);
		return new Set(Array.isArray(obj) ? obj : []);
	} catch {
		return new Set();
	}
}

function saveDismissedIds(ids: Set<string>) {
	if (typeof localStorage === 'undefined') return;
	try {
		localStorage.setItem(DISMISS_KEY, JSON.stringify([...ids]));
	} catch {
		// ignore
	}
}

// ── Alert Banner（display_type = 2）────────────────────────────

function AlertBanner({ ann, onDismiss }: { ann: Announcement; onDismiss: () => void }) {
	return (
		<Alert variant="warning">
			<Megaphone className="mt-0.5 shrink-0" size={20} aria-hidden="true" />
			<div className="min-w-0 flex-1">
				<AlertTitle>{ann.title}</AlertTitle>
				<AlertDescription className="mt-0.5 line-clamp-2">{ann.content}</AlertDescription>
			</div>
			<Button variant="ghost" size="xs" round className="shrink-0" aria-label="关闭" onClick={onDismiss}>
				<X size={14} aria-hidden="true" />
			</Button>
		</Alert>
	);
}

// ── Modal Dialog（display_type = 3）───────────────────────────

function ModalDialog({ ann, onDismiss }: { ann: Announcement; onDismiss: () => void }) {
	return (
		<Dialog open onOpenChange={(next) => !next && onDismiss()}>
			<DialogContent aria-describedby={undefined} showClose={false}>
				<DialogHeader>
					<div className="flex items-start gap-3">
						<Info className="mt-1 shrink-0 text-info" size={24} aria-hidden="true" />
						<DialogTitle className="flex-1">{ann.title}</DialogTitle>
					</div>
				</DialogHeader>
				<Prose className="whitespace-pre-wrap py-2">{ann.content}</Prose>
				<DialogFooter>
					<Button variant="primary" onClick={onDismiss}>
						知道了
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}

// ── Iframe Modal（display_type = 4）───────────────────────────

function IframeModal({ ann, onDismiss }: { ann: Announcement; onDismiss: () => void }) {
	return (
		<Dialog open onOpenChange={(next) => !next && onDismiss()}>
			<DialogContent
				aria-describedby={undefined}
				showClose={false}
				className="w-[calc(100vw-2rem)] max-w-4xl gap-0 overflow-hidden p-0"
			>
				<div className="flex items-center justify-between border-b border-border px-4 py-3">
					<DialogTitle>{ann.title}</DialogTitle>
					<Button variant="ghost" size="xs" round aria-label="关闭" onClick={onDismiss}>
						<X size={16} aria-hidden="true" />
					</Button>
				</div>
				{ann.content ? (
					<iframe
						src={ann.content}
						className="w-full border-0"
						style={{ height: '70vh' }}
						sandbox="allow-scripts allow-same-origin allow-forms allow-popups"
						title={ann.title}
					/>
				) : null}
			</DialogContent>
		</Dialog>
	);
}

// ── 公告状态（横幅与弹窗共用） ───────────────────────────────

function useAnnouncementState() {
	const { announcements } = useAnnouncements();
	const [dismissed, setDismissed] = useState<Set<string>>(() => new Set());
	const [mounted, setMounted] = useState(false);

	// 挂载后才读 localStorage：SSR 读不到，首帧必须与服务器 HTML 一致
	useEffect(() => {
		setMounted(true);
		setDismissed(getDismissedIds());
	}, []);

	useEffect(() => {
		if (dismissed.size > 0) saveDismissedIds(dismissed);
	}, [dismissed]);

	const dismiss = useCallback((id: string) => {
		setDismissed((prev) => new Set(prev).add(id));
	}, []);

	// 按 display_type 分流：1 toast / 2 横幅 / 3·4 弹窗
	const toastAnnouncements = useMemo(
		() => announcements.filter((a) => a.display_type === 1 && !dismissed.has(a.id)),
		[announcements, dismissed],
	);
	const alertAnnouncements = useMemo(
		() => announcements.filter((a) => a.display_type === 2 && !dismissed.has(a.id)),
		[announcements, dismissed],
	);
	const modalAnnouncements = useMemo(
		() => announcements.filter((a) => (a.display_type === 3 || a.display_type === 4) && !dismissed.has(a.id)),
		[announcements, dismissed],
	);

	return { mounted, dismiss, toastAnnouncements, alertAnnouncements, modalAnnouncements, setDismissed };
}

// ── 内联横幅（走文档流，占位不遮挡） ─────────────────────────

/**
 * 横幅属于页面内容：必须参与文档流，否则会压住页头、并在滚动时一直盖着正文。
 * 因此它不再走 portal，而是由 AppShell 放在 Nav 之后、主列之前。
 */
export function AnnouncementBanner() {
	const { mounted, dismiss, alertAnnouncements } = useAnnouncementState();

	if (!mounted || alertAnnouncements.length === 0) return null;

	return (
		<div className="w-full space-y-2 px-4 pt-3">
			{alertAnnouncements.map((ann) => (
				<AlertBanner key={ann.id} ann={ann} onDismiss={() => dismiss(ann.id)} />
			))}
		</div>
	);
}

// ── 浮层（弹窗 + toast 播报） ────────────────────────────────

export function AnnouncementOverlay() {
	const { mounted, dismiss, toastAnnouncements, modalAnnouncements, setDismissed } = useAnnouncementState();
	const [showModal, setShowModal] = useState(true);

	const closeModal = useCallback(() => {
		// 一次关掉当前所有可见弹窗类公告
		setDismissed((prev) => {
			const next = new Set(prev);
			for (const a of modalAnnouncements) next.add(a.id);
			return next;
		});
		setShowModal(false);
	}, [modalAnnouncements, setDismissed]);

	// toast 型只播报一次，播完即记为已读（旧 createEffect + 模块级 firedToastIds 集合的等价物）
	const firedToastIds = useRef(new Set<string>());
	useEffect(() => {
		for (const ann of toastAnnouncements) {
			if (firedToastIds.current.has(ann.id)) continue;
			firedToastIds.current.add(ann.id);
			toast.info(ann.title, 5000);
			dismiss(ann.id);
		}
	}, [toastAnnouncements, dismiss]);

	if (!mounted) return null;

	const currentModal = modalAnnouncements[0];

	return createPortal(
		<>
			{/* Modal — 第一条未读的弹窗/iframe 公告。key 让切换公告时重建弹层（旧 Show 靠重建复位） */}
			{showModal && currentModal ? (
				currentModal.display_type === 4 ? (
					<IframeModal key={currentModal.id} ann={currentModal} onDismiss={closeModal} />
				) : (
					<ModalDialog key={currentModal.id} ann={currentModal} onDismiss={closeModal} />
				)
			) : null}
		</>,
		document.body,
	);
}
