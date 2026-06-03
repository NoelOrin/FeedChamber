import type { JSX } from "solid-js/jsx-runtime";
import { createSignal } from "solid-js";
import Nav from "~/components/layout/nav/Nav";
import Sidebar from "~/components/aside/Sidebar";
import BackToTop from "~/components/lc/BackToTop";
import Footer from "~/components/layout/Footer";
import SidebarContent from "~/components/aside/SidebarContent";
import ToastContainer from "~/components/ui/Toast";
import AnnouncementOverlay from "~/components/announcement/AnnouncementOverlay";

interface LayoutProps {
  children: JSX.Element;
}

export default function Layout(props: LayoutProps) {
  const [desktopCollapsed, setDesktopCollapsed] = createSignal(false);
  const [mobileOpen, setMobileOpen] = createSignal(false);

  const handleToggle = () => {
    if (typeof window !== "undefined" && window.innerWidth >= 1280) {
      setDesktopCollapsed((v) => !v);
    } else {
      setMobileOpen((v) => !v);
    }
  };

  return (
    <>
      <Nav onToggleSidebar={handleToggle} />
      <div class="mx-auto mt-12 py-1.5 min-h-[100vh]">
        <div class="flex w-full h-full justify-between px-2 space-x-2">
          <Sidebar
            desktopCollapsed={desktopCollapsed()}
            mobileOpen={mobileOpen()}
            onClose={() => setMobileOpen(false)}
          >
            <SidebarContent />
          </Sidebar>
          <main class="mx-0 flex flex-1 flex-col min-w-0">
            {props.children}
          </main>
        </div>
      </div>
      <BackToTop />
      <Footer />
      <AnnouncementOverlay />
      <ToastContainer />
    </>
  );
}
