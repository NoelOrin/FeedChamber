import { For } from "solid-js";
import { Portal } from "solid-js/web";
import { useToasts, type Toast } from "~/utils/toast";
import {
  CircleAlert,
  CircleCheck,
  CircleX,
  Info,
  X,
} from "lucide-solid";
import { toast } from "~/utils/toast";

const iconMap = {
  info: Info,
  success: CircleCheck,
  warning: CircleAlert,
  error: CircleX,
};

const alertClass = {
  info: "alert-info",
  success: "alert-success",
  warning: "alert-warning",
  error: "alert-error",
};

function ToastItem(props: { toast: Toast }) {
  const Icon = iconMap[props.toast.type];

  return (
    <div
      class={`alert ${alertClass[props.toast.type]} shadow-lg flex items-center toast-item-enter`}
      role="alert"
    >
      <Icon class="shrink-0" size={18} />
      <span class="flex-1 text-sm">{props.toast.message}</span>
      <button
        class="btn btn-ghost btn-xs btn-circle"
        onClick={() => toast.remove(props.toast.id)}
      >
        <X size={14} />
      </button>
    </div>
  );
}

const positionClass = {
  "top-center": "toast toast-top toast-center",
  "top-right": "toast toast-top toast-end",
  "bottom-center": "toast toast-bottom toast-center",
  "bottom-right": "toast toast-bottom toast-end",
};

export default function ToastContainer() {
  const { toasts, position } = useToasts();

  return (
    <Portal>
      <div class={positionClass[position()]}>
        <For each={toasts()}>
          {(t) => <ToastItem toast={t} />}
        </For>
      </div>
    </Portal>
  );
}
