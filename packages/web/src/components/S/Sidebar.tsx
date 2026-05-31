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
      <aside id="sidebar-left" class="hidden xl:block w-72">
        {props.children}
      </aside>

      {/* Mobile */}
      <Show when={props.isOpen}>
        <div class="select-none" id="mySidebar">
          <div id="mask" class="active" onClick={props.onClose} />
          <aside id="sidebar-left" class="drawer-active sidebar-left-pop">
            <div class="-mt-2">{props.children}</div>
          </aside>
        </div>
      </Show>
    </>
  );
}
