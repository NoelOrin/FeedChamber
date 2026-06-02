import { Route } from "~/pages/__root";

export default function SiteName() {
  const settings = Route.useLoaderData()().settings ?? {};
  const siteName = settings.site_name || "OrinForum";

  return (
    <div class="sm:block xl:hidden group/sitename relative">
      <a href="/">
        <div id="logo" class="ml-0 select-none">
          <div
            id="site-name"
            style="font-family: 'Segoe UI',Arial,'Microsoft Yahei',sans-serif;"
          >
            {siteName}
          </div>
          <div class="absolute flex icon py-1.5">
            {/** biome-ignore lint/a11y/noSvgWithoutTitle: <explanation> */}
            <svg
              fill="none"
              height="24"
              viewBox="0 0 48 48"
              width="24"
              xmlns="http://www.w3.org/2000/svg"
            >
              <path
                d="M44 44V20L24 4L4 20L4 44H16V26H32V44H44Z"
                fill="none"
                stroke="#ffffff"
                stroke-linejoin="round"
                stroke-width="5"
              />
            </svg>
          </div>
        </div>
      </a>
    </div>
  )
}
