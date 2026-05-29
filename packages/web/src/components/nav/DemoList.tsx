export default function DemoList() {
	return (
		<div class="mt-4 flex flex-col card">
			<div>
				<div class="ml-2 flex flex-row items-center text-xl demoHeaderTitle">
					<svg class="mr-2 inline-block mt-0.5" width="22" height="22" viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
						<path d="M12 4H36" stroke="currentColor" stroke-width="4" stroke-linecap="round" />
						<path d="M10.7769 30L18.019 15.0386V4H30.0283V15.0386L37.246 30" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
						<path d="M7.79433 43.673C6.16744 42.8855 5.48698 40.9282 6.27449 39.3013L10.7769 30C10.7769 30 18.0001 35 24.0001 30C30.0001 25 37.2461 30 37.2461 30L41.7352 39.3052C41.9492 39.7488 42.0603 40.2348 42.0603 40.7273C42.0603 42.5347 40.595 44 38.7876 44H9.22025C8.72636 44 8.23888 43.8882 7.79433 43.673Z" fill="none" stroke="currentColor" stroke-width="4" stroke-linejoin="round" />
					</svg>
					<div class="inline-block">
						<span>
							Demo<span class="text-sm ml-2.5">Web 小玩具</span>
						</span>
					</div>
				</div>
				<div class="mt-4 text-sm card-body">
					<ul>
						<li class="my-0.5">
							<span class="flex w-full cursor-pointer flex-row items-center justify-between truncate px-2 py-1 duration-200 demoItem">
								<span>123</span>
								<span class="p-1 text-xs demoItemNum"> </span>
							</span>
						</li>
					</ul>
				</div>
			</div>
		</div>
	)
}
