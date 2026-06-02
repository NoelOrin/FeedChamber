import { Link } from "@tanstack/solid-router";
import { Show } from "solid-js";
import { Route } from "~/pages/__root";

export default function SiteName() {
  const settings = Route.useLoaderData()().settings ?? {};
  const siteName = settings.site_name || "OrinForum";
  const siteLogo = settings.site_logo || "";

  return (
    <div class="group/sitename relative">
      <Link to="/">
        <div id="logo" class="ml-0 select-none flex items-center gap-2">
          <Show when={siteLogo}>
            <img
              src={siteLogo}
              alt={siteName}
              class="h-8 w-8 object-contain shrink-0"
            />
          </Show>
          <div
            id="site-name"
            class={siteLogo ? "hidden lg:block" : "block"}
            style="font-family: 'Segoe UI',Arial,'Microsoft Yahei',sans-serif;"
          >
            {siteName}
          </div>
          <div class="absolute flex icon py-1.5">
            {/** biome-ignore lint/a11y/noSvgWithoutTitle: <explanation> */}
            <svg
              fill="none"
              height="24"
              width="24"
              viewBox="0 0 48 48"
              xmlns="http://www.w3.org/2000/svg"
            >
              <path
                d="M44 44V20L24 4L4 20L4 44H16V26H32V44H44Z"
                fill="none"
                stroke="#ffffff"
                stroke-linejoin="round"
                stroke-width="4"
              />
            </svg>
          </div>
        </div>
      </Link>
    </div>
  );
}
