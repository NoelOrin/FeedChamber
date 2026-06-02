import { Show, type JSX } from "solid-js";

interface SidebarProps {
  isOpen?: boolean;
  onClose?: () => void;
  children?: JSX.Element;
}

export default function Sidebar(props: SidebarProps) {
  return (
    <>
      {/* Desktop */}
      <Show when={!props.isOpen}>
        <aside id="sidebar-left" class="hidden xl:block w-64">
          {props.children}
        </aside>
      </Show>
      {/* Mobile */}
      <Show when={props.isOpen}>
        <div class="select-none">
          <div id="mask" class="active" onClick={props.onClose} />
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
