import { createStore, useStore } from 'zustand';

type ToastType = 'info' | 'success' | 'warning' | 'error';
export type ToastPosition = 'top-center' | 'top-right' | 'bottom-center' | 'bottom-right';

export interface ToastItem {
	id: number;
	message: string;
	type: ToastType;
	duration: number;
}

interface ToastOptions {
	type?: ToastType;
	duration?: number;
}

interface ToastState {
	toasts: ToastItem[];
	position: ToastPosition;
}

const store = createStore<ToastState>(() => ({ toasts: [], position: 'top-center' }));

export function useToasts() {
	const toasts = useStore(store, (state) => state.toasts);
	const position = useStore(store, (state) => state.position);
	return { toasts, position };
}

let nextId = 0;

function remove(id: number) {
	store.setState((state) => ({ toasts: state.toasts.filter((t) => t.id !== id) }));
}

function add(message: string, options: ToastOptions = {}) {
	const id = nextId++;
	const duration = options.duration ?? 3000;
	store.setState((state) => ({
		toasts: [...state.toasts, { id, message, type: options.type ?? 'info', duration }],
	}));
	if (duration > 0) setTimeout(() => remove(id), duration);
	return id;
}

export const toast = Object.assign(add, {
	info: (message: string, duration?: number) => add(message, { type: 'info', duration }),
	success: (message: string, duration?: number) => add(message, { type: 'success', duration }),
	warning: (message: string, duration?: number) => add(message, { type: 'warning', duration }),
	error: (message: string, duration?: number) => add(message, { type: 'error', duration }),
	remove,
	setPosition: (position: ToastPosition) => store.setState({ position }),
});
