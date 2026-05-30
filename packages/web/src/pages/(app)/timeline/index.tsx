import { createFileRoute } from "@tanstack/solid-router";
import Paginator from "~/components/pagination/Paginator";
import ScrollPaginator from "~/components/pagination/ScrollPaginator";

export const Route = createFileRoute("/(app)/timeline/")({
  component: Home,
});

function Home() {
  return (
    <div class="w-full h-full flex flex-col">
      <div class="grid gap-4 lg:grid-cols-1 xl:grid-cols-2"></div>
      {/* Pagination */}
      {/* <Paginator pageNum={1} totalPages={10} /> */}
      <ScrollPaginator hasMore={true} isLoading={false} onLoadMore={() => {}} />
    </div>
  );
}
