import { Link } from "@tanstack/solid-router";
import { For, Show } from "solid-js";
import type { Tag } from "~/api";

interface TagsProps {
  tags?: Tag[];
  class?: string;
}

export default function TagsAside(props: TagsProps) {
  return (
    <div class={`flex flex-col card ${props.class || ""}`}>
      <div class="flex items-center text-xl cardHeaderTitle">
        <svg
          class="mr-2 inline-block"
          width="24"
          height="24"
          viewBox="0 0 48 48"
          fill="none"
          xmlns="http://www.w3.org/2000/svg"
        >
          <path
            d="M42.1691 29.2451L29.2631 42.1511C28.5879 42.8271 27.6716 43.2069 26.7161 43.2069C25.7606 43.2069 24.8444 42.8271 24.1691 42.1511L8 26V8H26L42.1691 24.1691C43.5649 25.5732 43.5649 27.841 42.1691 29.2451Z"
            fill="none"
            stroke="currentColor"
            stroke-width="4"
            stroke-linejoin="round"
          />
          <path
            fill-rule="evenodd"
            clip-rule="evenodd"
            d="M18.5 21C19.8807 21 21 19.8807 21 18.5C21 17.1193 19.8807 16 18.5 16C17.1193 16 16 17.1193 16 18.5C16 19.8807 17.1193 21 18.5 21Z"
            fill="currentColor"
          />
        </svg>
        <div class="inline-block">
          <span>
            {/* Tag */}
            <span class="text-base">标签</span>
          </span>
        </div>
      </div>
      <div class="text-sm card-body p-2 mt-1.5">
        <Show when={props.tags}>
          <div class="flex flex-wrap gap-2">
            <For each={props.tags}>
              {(tag) => (
                <Link
                  to="/timeline"
                  search={() => ({
                    // ...prev,
                    tag: tag.name,
                    category: undefined,
                  })}
                  class="flex cursor-pointer flex-row items-start px-2 card py-1.5 bg-gray-100 dark:bg-zinc-600 hover:bg-gray-100 dark:hover:bg-zinc-600"
                >
                  <div>{tag.name}</div>
                </Link>
              )}
            </For>
          </div>
        </Show>
      </div>
    </div>
  );
}
