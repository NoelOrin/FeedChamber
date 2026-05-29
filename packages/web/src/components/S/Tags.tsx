import { createResource } from 'solid-js'

interface TagItem {
	tagName: string
	tagCount: number
}

interface TagsResponse {
	data: {
		tags: TagItem[]
	}
}

async function fetchTags(): Promise<TagsResponse> {
	const res = await fetch('/api/v1/tags')
	if (!res.ok) throw new Error('Failed to fetch tags')
	return res.json()
}

export default function Tags() {
	const [data] = createResource(fetchTags)

	return (
		<div class="mt-4 flex flex-col card">
			<div class="ml-2 flex flex-row items-center text-xl cardHeaderTitle">
				<svg class="mr-2 inline-block" width="24" height="24" viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
					<path d="M42.1691 29.2451L29.2631 42.1511C28.5879 42.8271 27.6716 43.2069 26.7161 43.2069C25.7606 43.2069 24.8444 42.8271 24.1691 42.1511L8 26V8H26L42.1691 24.1691C43.5649 25.5732 43.5649 27.841 42.1691 29.2451Z" fill="none" stroke="currentColor" stroke-width="4" stroke-linejoin="round" />
					<path fill-rule="evenodd" clip-rule="evenodd" d="M18.5 21C19.8807 21 21 19.8807 21 18.5C21 17.1193 19.8807 16 18.5 16C17.1193 16 16 17.1193 16 18.5C16 19.8807 17.1193 21 18.5 21Z" fill="currentColor" />
				</svg>
				<div class="inline-block">
					<span>
						Tag<span class="ml-2 text-base">标签</span>
					</span>
				</div>
			</div>
			<div class="mt-2 card-body">
				<div class="flex flex-wrap space-x-2">
					<For each={data()?.data?.tags}>
						{(tag) => (
							<a class="flex cursor-pointer flex-row items-start px-2 card py-1.5 tag space-x-1.5" href={`/?page=1&tag=${tag.tagName}`}>
								<div>{tag.tagName}</div>
								<div class="text-xs opacity-75">{tag.tagCount}</div>
							</a>
						)}
					</For>
				</div>
			</div>
			<style>{`
				.tag { --tw-bg-opacity: 1; background-color: var(--light-item-color); }
				:is(.dark .tag) { --tw-bg-opacity: 1; background-color: var(--dark-item-color); }
				.tag:hover { --tw-bg-opacity: 1; background-color: var(--light-item-color); }
				:is(.dark .tag:hover) { --tw-bg-opacity: 1; background-color: var(--dark-item-color); }
			`}</style>
		</div>
	)
}
