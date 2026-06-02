import { createSignal } from "solid-js";

export type ToastType = "info" | "success" | "warning" | "error";
export type ToastPosition =
  | "top-center"
  | "top-right"
  | "bottom-center"
  | "bottom-right";

export interface Toast {
  id: number;
  message: string;
  type: ToastType;
  duration: number;
}

interface ToastOptions {
  type?: ToastType;
  duration?: number;
  position?: ToastPosition;
}

let nextId = 0;
const [toasts, setToasts] = createSignal<Toast[]>([]);
const [position, setPosition] = createSignal<ToastPosition>("top-center");

export function useToasts() {
  return { toasts, position };
}

function add(message: string, opts: ToastOptions = {}) {
  const id = nextId++;
  const toast: Toast = {
    id,
    message,
    type: opts.type ?? "info",
    duration: opts.duration ?? 3000,
  };

  setToasts((prev) => [...prev, toast]);

  if (toast.duration > 0) {
    setTimeout(() => remove(id), toast.duration);
  }

  return id;
}

function remove(id: number) {
  setToasts((prev) => prev.filter((t) => t.id !== id));
}

export const toast = Object.assign(add, {
  info: (msg: string, duration?: number) =>
    add(msg, { type: "info", duration }),
  success: (msg: string, duration?: number) =>
    add(msg, { type: "success", duration }),
  warning: (msg: string, duration?: number) =>
    add(msg, { type: "warning", duration }),
  error: (msg: string, duration?: number) =>
    add(msg, { type: "error", duration }),
  remove,
  setPosition,
});
