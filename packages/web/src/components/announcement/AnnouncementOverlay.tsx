import { createEffect, createSignal, For, onMount, Show } from "solid-js";
import { Portal } from "solid-js/web";
import { X, Megaphone, Info, AlertTriangle } from "lucide-solid";
import { announcementService, type Announcement } from "~/api/service/announcement";

const DISMISS_KEY = "announcement_dismissed";

function getDismissedIds(): Set<string> {
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
  try {
    localStorage.setItem(DISMISS_KEY, JSON.stringify([...ids]));
  } catch {
    // ignore
  }
}

// ── Alert Banner ──────────────────────────────────────────────

function AlertBanner(props: { ann: Announcement; onDismiss: () => void }) {
  return (
    <div class="alert alert-warning shadow-lg rounded-lg flex items-start gap-3 py-3 px-4">
      <Megaphone class="shrink-0 mt-0.5" size={20} />
      <div class="flex-1 min-w-0">
        <div class="font-semibold text-sm">{props.ann.title}</div>
        <div class="text-xs opacity-80 line-clamp-2 mt-0.5">
          {props.ann.content}
        </div>
      </div>
      <button
        class="btn btn-ghost btn-xs btn-circle shrink-0"
        onClick={props.onDismiss}
      >
        <X size={14} />
      </button>
    </div>
  );
}

// ── Modal Dialog ──────────────────────────────────────────────

function ModalDialog(props: { ann: Announcement; onDismiss: () => void }) {
  let dialogRef: HTMLDialogElement | undefined;

  onMount(() => {
    dialogRef?.showModal();
  });

  return (
    <dialog
      ref={dialogRef}
      class="modal modal-open"
      onClose={props.onDismiss}
    >
      <div class="modal-box max-w-lg">
        <div class="flex items-start gap-3 mb-3">
          <Info class="shrink-0 mt-1 text-info" size={24} />
          <h3 class="font-bold text-lg flex-1">{props.ann.title}</h3>
        </div>
        <div class="py-2 prose prose-sm max-w-none whitespace-pre-wrap break-words text-base-content/80">
          {props.ann.content}
        </div>
        <div class="modal-action">
          <form method="dialog">
            <button class="btn btn-primary" onClick={props.onDismiss}>
              知道了
            </button>
          </form>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop">
        <button onClick={props.onDismiss}>close</button>
      </form>
    </dialog>
  );
}

// ── Main Overlay ──────────────────────────────────────────────

export default function AnnouncementOverlay() {
  const [announcements, setAnnouncements] = createSignal<Announcement[]>([]);
  const [dismissed, setDismissed] = createSignal<Set<string>>(new Set());
  const [showModal, setShowModal] = createSignal(true);

  onMount(async () => {
    setDismissed(getDismissedIds());
    try {
      const res = await announcementService.listPublished();
      setAnnouncements(res.list || []);
    } catch (e) {
      console.error("Failed to load announcements:", e);
    }
  });

  const dismiss = (id: string) => {
    const next = new Set([...dismissed(), id]);
    setDismissed(next);
    saveDismissedIds(next);
  };

  const closeModal = () => {
    // dismiss all visible modals
    const next = new Set(dismissed());
    for (const a of modalAnnouncements()) {
      next.add(a.id);
    }
    setDismissed(next);
    saveDismissedIds(next);
    setShowModal(false);
  };

  // Split announcements by display_type
  const toastAnnouncements = () =>
    announcements().filter(
      (a) => a.display_type === 1 && !dismissed().has(a.id),
    );
  const alertAnnouncements = () =>
    announcements().filter(
      (a) => a.display_type === 2 && !dismissed().has(a.id),
    );
  const modalAnnouncements = () =>
    announcements().filter(
      (a) => a.display_type === 3 && !dismissed().has(a.id),
    );

  // Fire toasts for toast-type announcements once
  let firedToastIds = new Set<string>();
  createEffect(() => {
    for (const ann of toastAnnouncements()) {
      if (firedToastIds.has(ann.id)) continue;
      firedToastIds.add(ann.id);
      import("~/utils/toast").then(({ toast }) => {
        toast.info(ann.title, 5000);
      });
      // auto-dismiss after toast fires
      dismiss(ann.id);
    }
  });

  return (
    <Portal>
      {/* Alert banners — top of page */}
      <Show when={alertAnnouncements().length > 0}>
        <div class="fixed top-14 left-0 right-0 z-[100] px-4 space-y-2 pointer-events-none">
          <div class="max-w-3xl mx-auto space-y-2 pointer-events-auto">
            <For each={alertAnnouncements()}>
              {(ann) => <AlertBanner ann={ann} onDismiss={() => dismiss(ann.id)} />}
            </For>
          </div>
        </div>
      </Show>

      {/* Modal — first unread modal announcement */}
      <Show when={showModal() && modalAnnouncements().length > 0}>
        <ModalDialog
          ann={modalAnnouncements()[0]}
          onDismiss={closeModal}
        />
      </Show>
    </Portal>
  );
}
