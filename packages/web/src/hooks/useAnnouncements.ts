import { createSignal, onMount } from "solid-js";
import { announcementService, type Announcement } from "~/api/service/announcement";

export function useAnnouncements() {
  const [announcements, setAnnouncements] = createSignal<Announcement[]>([]);
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await announcementService.listPublished();
      setAnnouncements(res.list || []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load announcements");
    } finally {
      setLoading(false);
    }
  };

  onMount(() => {
    load();
  });

  return {
    announcements,
    loading,
    error,
    refresh: load,
  };
}
