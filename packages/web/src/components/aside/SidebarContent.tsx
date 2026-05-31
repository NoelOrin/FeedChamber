import { Route as RootRoute } from "~/pages/__root";
import CategoryAside from "./Category";
import TagsAside from "./Tags";

const SidebarContent = () => {
  const loaderData = RootRoute.useLoaderData();

  return (
    <div class="sticky top-13.5 flex flex-col space-y-2">
      <CategoryAside class="p-3" categories={loaderData().categories} />
      <TagsAside class="p-3" tags={loaderData().tags} />
    </div>
  );
};

export default SidebarContent;
