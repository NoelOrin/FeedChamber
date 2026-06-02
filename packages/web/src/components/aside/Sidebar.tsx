import { Show, type JSX } from "solid-js";

interface SidebarProps {
  desktopCollapsed?: boolean;
  mobileOpen?: boolean;
  onClose?: () => void;
  children?: JSX.Element;
}

export default function Sidebar(props: SidebarProps) {
  return (
    <>
      {/* Desktop xl+: 默认显示，collapsed 时隐藏 */}
      <Show when={!props.desktopCollapsed}>
        <aside id="sidebar-left" class="hidden xl:block w-64">
          {props.children}
        </aside>
      </Show>

      {/* Mobile < xl: 点击汉堡按钮后以 drawer 展开 */}
      <Show when={props.mobileOpen}>
        <div class="select-none">
          <div
            class="fixed inset-0 z-40 bg-white/60 dark:bg-zinc-700/50 backdrop-blur-[8px]"
            onClick={props.onClose}
          />
          <aside
            id="sidebar-left"
            class="drawer-active sidebar-left-pop w-64 xl:hidden block"
          >
            <div class="-mt-2">{props.children}</div>
          </aside>
        </div>
      </Show>
    </>
  );
}
