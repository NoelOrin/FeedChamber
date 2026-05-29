import type { JSX } from "solid-js/jsx-runtime";
import { createSignal } from "solid-js";
import Nav from "~/components/layout/nav/Nav";
import Sidebar from "~/components/S/Sidebar";
import SidebarPop from "~/components/S/SidebarPop";
import BackToTop from "~/components/lc/BackToTop";
import Footer from "~/components/layout/Footer";

interface LayoutProps {
  children: JSX.Element;
}

export default function Layout(props: LayoutProps) {
  const [sidebarOpen, setSidebarOpen] = createSignal(false);

  return (
    <div>
      <Nav onToggleSidebar={() => setSidebarOpen(!sidebarOpen())} />
      <div class="mx-auto mt-10 flex-col justify-between py-1.5 min-h-[90vh] sm:max-w-[1080px] xl:max-w-[1080px] 2xl:max-w-[1248px]">
        <div class="flex w-full h-full max-w-full">
          <Sidebar />
          <SidebarPop
            isOpen={sidebarOpen()}
            onClose={() => setSidebarOpen(false)}
          />
          <main class="mx-0 flex flex-1 flex-col p-2 min-w-0">
            {props.children}
            <BackToTop />
          </main>
        </div>
      </div>
      <Footer />
    </div>
  );
}
