import { createSignal, onMount, JSX, Show } from "solid-js"

interface ClientOnlyProps {
  fallback?: JSX.Element
  children: JSX.Element
}

export default function ClientOnly(props: ClientOnlyProps) {
  const [mounted, setMounted] = createSignal(false)
  onMount(() => setMounted(true))
  return <Show when={mounted()} fallback={props.fallback}>{props.children}</Show>
}
