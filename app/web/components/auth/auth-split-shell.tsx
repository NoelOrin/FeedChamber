import Link from 'next/link';

import type { LucideIcon } from '~/lib/icons';
import { Archive, MessageSquare, Users } from '~/lib/icons';

const CURRENT_YEAR = new Date().getFullYear();

/* 左栏的固定文案：站点名与简介来自设置，这三条是认证页自带的产品气质说明。 */
const HIGHLIGHTS: { icon: LucideIcon; title: string; description: string }[] = [
	{ icon: MessageSquare, title: '专注讨论', description: '没有噪音，只有内容本身。' },
	{ icon: Archive, title: '长期沉淀', description: '好的回答不会被时间冲走。' },
	{ icon: Users, title: '找到同好', description: '与在意同一件事的人相遇。' },
];

/** 左栏字标：品牌色面上用半透明白块托底，与主站 Nav 的字标保持同一形状。 */
function BrandMark({ siteName, siteLogo }: { siteName: string; siteLogo?: string }) {
	return (
		<>
			<span className="grid size-8 shrink-0 place-items-center overflow-hidden rounded-md bg-primary-foreground/15 text-primary-foreground">
				{siteLogo ? (
					<img src={siteLogo} alt="" className="size-8 object-cover" />
				) : (
					<MessageSquare size={16} aria-hidden="true" />
				)}
			</span>
			<span className="truncate text-sm font-semibold tracking-tight">{siteName}</span>
		</>
	);
}

/**
 * 认证页的左右分栏外壳：左栏是品牌装饰面，右栏是表单。
 * 左栏在 md 及以上出现（平板起就分栏；手机竖屏不浪费首屏高度），右栏在窄屏撑满整屏。
 */
export function AuthSplitShell({
	siteName,
	siteDescription,
	siteLogo,
	title,
	subtitle,
	children,
	footer,
}: {
	siteName: string;
	siteDescription?: string;
	siteLogo?: string;
	title: string;
	subtitle?: string;
	children: React.ReactNode;
	footer?: React.ReactNode;
}) {
	return (
		<div className="grid min-h-screen md:grid-cols-[1.1fr_1fr]">
			{/* ——— 左栏：装饰面 ——— */}
			<section className="relative hidden overflow-hidden bg-primary p-8 text-primary-foreground md:flex md:flex-col md:justify-between lg:p-10 xl:p-14">
				{/* 压暗一层：品牌紫在暗色主题下偏亮，白字小号文案对比不足；纯色叠加不引入渐变 */}
				<div aria-hidden="true" className="pointer-events-none absolute inset-0 bg-black/20" />
				{/* 同心圆环：纯线稿，贴角出血，只作背景层次 */}
				<div aria-hidden="true" className="pointer-events-none absolute inset-0">
					<div className="absolute -top-40 -left-40 size-[420px] rounded-full border border-primary-foreground/[0.12]" />
					<div className="absolute -right-56 -bottom-56 size-[760px] rounded-full border border-primary-foreground/[0.14]" />
					<div className="absolute -right-40 -bottom-40 size-[560px] rounded-full border border-primary-foreground/[0.18]" />
					<div className="absolute -right-24 -bottom-24 size-[380px] rounded-full border border-primary-foreground/[0.22]" />
				</div>

				<Link href="/" className="relative inline-flex items-center gap-2 transition-opacity hover:opacity-80">
					<BrandMark siteName={siteName} siteLogo={siteLogo} />
				</Link>

				<div className="relative my-10 max-w-[27rem]">
					{/* 装饰性标语，不用标题元素：页面唯一的 h1 属于右栏的表单标题 */}
					<p className="text-[28px] leading-[1.28] font-semibold tracking-tight xl:text-[34px]">
						讨论有价值的话题
						<span className="mt-1.5 block text-primary-foreground/75">沉淀可检索的知识</span>
					</p>
					<p className="mt-5 max-w-[24rem] text-[13px] leading-6 text-primary-foreground/85">
						{siteDescription || `${siteName} 是一个专注讨论与分享的社区。`}
					</p>

					<ul className="mt-10 flex flex-col gap-5">
						{HIGHLIGHTS.map(({ icon: Icon, title: itemTitle, description }) => (
							<li key={itemTitle} className="flex items-start gap-3">
								<span className="grid size-8 shrink-0 place-items-center rounded-md bg-primary-foreground/15">
									<Icon size={15} aria-hidden="true" />
								</span>
								<span className="flex flex-col gap-0.5">
									<span className="text-[13px] font-medium">{itemTitle}</span>
									<span className="text-xs leading-5 text-primary-foreground/85">{description}</span>
								</span>
							</li>
						))}
					</ul>
				</div>

				<p className="relative text-[11px] text-primary-foreground/85">
					&copy; {CURRENT_YEAR} {siteName}
				</p>
			</section>

			{/* ——— 右栏：表单 ——— */}
			<section className="flex flex-col bg-background px-5 py-8 sm:px-8 md:px-8 lg:px-12 lg:py-10">
				{/* 页头与页脚收进同一个 max-w 容器，窄屏时与表单左边界对齐 */}
				<div className="mx-auto flex w-full max-w-[400px] flex-1 flex-col">
					<Link
						href="/"
						className="mb-10 inline-flex items-center gap-2 self-start text-foreground transition-opacity hover:opacity-80 md:hidden"
					>
						<span className="grid size-8 shrink-0 place-items-center rounded-md bg-primary text-primary-foreground">
							{siteLogo ? (
								<img src={siteLogo} alt="" className="size-8 rounded-md object-cover" />
							) : (
								<MessageSquare size={16} aria-hidden="true" />
							)}
						</span>
						<span className="truncate text-sm font-semibold tracking-tight">{siteName}</span>
					</Link>

					<div className="flex flex-1 flex-col justify-center">
						<h1 className="text-2xl leading-tight font-semibold tracking-tight text-foreground">{title}</h1>
						{subtitle ? <p className="mt-2 text-[13px] text-muted-foreground">{subtitle}</p> : null}

						<div className="mt-8">{children}</div>

						{footer ? (
							<div className="mt-8 border-t border-border/60 pt-5 text-center text-[13px]">{footer}</div>
						) : null}
					</div>

					<p className="mt-10 text-center text-[11px] text-muted-foreground">
						&copy; {CURRENT_YEAR} {siteName}
					</p>
				</div>
			</section>
		</div>
	);
}
