import { Link } from "@tanstack/solid-router";
import { For, Show } from "solid-js";
import type { Category } from "~/api";

interface CategoryProps {
  categories?: Category[];
  class?: string;
}

export default function CategoryAside(props: CategoryProps) {
  return (
    <div class={`flex flex-col card ${props.class || ""}`}>
      <div class="flex items-center text-xl cardHeaderTitle">
        <svg
          class="inline-block"
          width="22"
          height="22"
          viewBox="0 0 48 48"
          fill="none"
          xmlns="http://www.w3.org/2000/svg"
        >
          <path
            d="M8 44L8 6C8 4.89543 8.89543 4 10 4H38C39.1046 4 40 4.89543 40 6V44L24 35.7273L8 44Z"
            fill="none"
            stroke="currentColor"
            stroke-width="4"
            stroke-linejoin="round"
          />
          <path
            d="M16 18H32"
            stroke="currentColor"
            stroke-width="4"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
        <div class="inline-block">
          <span>
            {/* Category */}
            <span class="ml-2 text-base">分类</span>
          </span>
        </div>
      </div>
      <div class="text-sm card-body p-2 mt-1.5">
        <Show when={props.categories}>
          <ul>
            <For each={props.categories}>
              {(item) => (
                <li class="my-0.5">
                  <Link
                    to="/timeline"
                    search={() => ({ category: item.name, tag: undefined })}
                  >
                    <span class="flex w-full cursor-pointer flex-row items-center justify-between truncate px-2 py-1 duration-200 rounded-[0.15rem] hover:bg-gray-100 dark:hover:bg-zinc-600">
                      <span>{item.name}</span>
                    </span>
                  </Link>
                </li>
              )}
            </For>
          </ul>
        </Show>
      </div>
    </div>
  );
}
