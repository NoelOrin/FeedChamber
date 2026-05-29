export default function Friends() {
	return (
		<div class="mt-4 flex flex-col card">
			<div class="ml-2 flex flex-row items-center text-xl cardHeaderTitle">
				<svg class="mr-2 inline-block" width="24" height="24" viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
					<circle cx="14" cy="29" r="5" fill="none" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
					<circle cx="34" cy="29" r="5" fill="none" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
					<circle cx="24" cy="9" r="5" fill="none" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
					<path d="M24 44C24 38.4772 19.5228 34 14 34C8.47715 34 4 38.4772 4 44" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
					<path d="M44 44C44 38.4772 39.5228 34 34 34C28.4772 34 24 38.4772 24 44" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
					<path d="M34 24C34 18.4772 29.5228 14 24 14C18.4772 14 14 18.4772 14 24" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
				</svg>
				<div class="inline-block">
					<span>
						Friend<span class="ml-2 text-base">友链</span>
					</span>
				</div>
			</div>
			<div class="mt-4 text-sm card-body" />
		</div>
	)
}
