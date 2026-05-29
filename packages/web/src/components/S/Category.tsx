import { createResource, For } from 'solid-js'

interface CategoryItem {
	name: string
	total: number
}

interface CategoryResponse {
	data: {
		total: number
		categories: CategoryItem[]
	}
}

async function fetchCategories(): Promise<CategoryResponse> {
	const res = await fetch('/api/v1/category')
	if (!res.ok) throw new Error('Failed to fetch categories')
	return res.json()
}

export default function Category() {
	const [data] = createResource(fetchCategories)

	return (
		<div class="mt-4 flex flex-col card">
			<div class="ml-2 flex flex-row items-end text-xl cardHeaderTitle">
				<svg class="mr-2 inline-block mb-0.5" width="22" height="22" viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
					<path d="M8 44L8 6C8 4.89543 8.89543 4 10 4H38C39.1046 4 40 4.89543 40 6V44L24 35.7273L8 44Z" fill="none" stroke="currentColor" stroke-width="4" stroke-linejoin="round" />
					<path d="M16 18H32" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
				</svg>
				<div class="inline-block">
					<span>
						Category<span class="ml-2 text-base">分类</span>
					</span>
				</div>
			</div>
			<div class="mt-4 text-sm card-body">
				<ul>
					<li class="my-0.5">
						<a href="/?page=1">
							<span class="flex w-full cursor-pointer flex-row items-center justify-between truncate px-2 py-1 duration-200 categoryItem">
								<span>全部文章</span>
								<span class="px-2 py-1 text-xs categoryItemNum">{data()?.data.total ?? ''}</span>
							</span>
						</a>
					</li>
					<For each={data()?.data.categories}>
						{(item) => (
							<li class="my-0.5">
								<a href={`/?page=1&category=${item.name}`}>
									<span class="flex w-full cursor-pointer flex-row items-center justify-between truncate px-2 py-1 duration-200 categoryItem">
										<span>{item.name}</span>
										<span class="px-2 py-1 text-xs categoryItemNum">{item.total}</span>
									</span>
								</a>
							</li>
						)}
					</For>
				</ul>
			</div>
			<style>{`
				.categoryItem { border-radius: 0.15rem; }
				.categoryItem:hover { --tw-bg-opacity: 1; background-color: rgb(243 244 246 / var(--tw-bg-opacity)); }
				:is(.dark .categoryItem:hover) { --tw-bg-opacity: 1; background-color: rgb(82 82 91 / var(--tw-bg-opacity)); }
				.categoryItemNum { border-radius: .15rem; --tw-bg-opacity: 1; background-color: rgb(243 244 246 / var(--tw-bg-opacity)); }
				:is(.dark .categoryItemNum) { --tw-bg-opacity: 1; background-color: rgb(82 82 91 / var(--tw-bg-opacity)); }
			`}</style>
		</div>
	)
}
