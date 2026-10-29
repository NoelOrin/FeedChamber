import type { CSSProperties, ReactNode } from 'react';

import {
	Book,
	Bug,
	Cloud,
	Coffee,
	Database,
	FileText,
	Flame,
	Folder,
	FolderOpen,
	Gamepad2,
	Globe,
	Heart,
	Laptop,
	Appstore as LayoutGrid,
	Megaphone,
	MessageCircle,
	MessageSquare,
	Monitor,
	Music,
	Rocket,
	Settings,
	ShoppingBag,
	Smartphone,
	Star,
	Trophy,
	Users,
	Wrench,
	Zap,
} from '~/lib/icons';

/*
 * 版面图标：后端 `space.icon` 历史数据是 emoji（📱 💻 ⚙️ …），后台一律渲染成 SVG 图标，
 * 不再把 emoji 字面量画到页面上。取值顺序：已知 emoji → 已知图标名 → 按层级取 Folder。
 *
 * 图标来源从 @ant-design/icons 换成 lucide（与主站同一套）。
 */
const sz = 'size-4';

const BY_NAME: Record<string, ReactNode> = {
	smartphone: <Smartphone className={sz} />,
	phone: <Smartphone className={sz} />,
	laptop: <Laptop className={sz} />,
	computer: <Monitor className={sz} />,
	monitor: <Monitor className={sz} />,
	settings: <Settings className={sz} />,
	gear: <Settings className={sz} />,
	message: <MessageSquare className={sz} />,
	chat: <MessageCircle className={sz} />,
	comment: <MessageSquare className={sz} />,
	server: <Cloud className={sz} />,
	cloud: <Cloud className={sz} />,
	database: <Database className={sz} />,
	code: <FileText className={sz} />,
	bug: <Bug className={sz} />,
	book: <Book className={sz} />,
	music: <Music className={sz} />,
	game: <Gamepad2 className={sz} />,
	rocket: <Rocket className={sz} />,
	star: <Star className={sz} />,
	heart: <Heart className={sz} />,
	flame: <Flame className={sz} />,
	trophy: <Trophy className={sz} />,
	users: <Users className={sz} />,
	tag: <FileText className={sz} />,
	folder: <Folder className={sz} />,
	'folder-open': <FolderOpen className={sz} />,
	file: <FileText className={sz} />,
	coffee: <Coffee className={sz} />,
	wrench: <Wrench className={sz} />,
	zap: <Zap className={sz} />,
	globe: <Globe className={sz} />,
	megaphone: <Megaphone className={sz} />,
	shopping: <ShoppingBag className={sz} />,
	app: <LayoutGrid className={sz} />,
};

const BY_EMOJI: Record<string, ReactNode> = {
	'📱': <Smartphone className={sz} />,
	'💻': <Laptop className={sz} />,
	'🖥': <Monitor className={sz} />,
	'⚙': <Settings className={sz} />,
	'🔧': <Wrench className={sz} />,
	'💬': <MessageSquare className={sz} />,
	'🗨': <MessageCircle className={sz} />,
	'📁': <Folder className={sz} />,
	'📂': <FolderOpen className={sz} />,
	'🗂': <FolderOpen className={sz} />,
	'📄': <FileText className={sz} />,
	'📝': <FileText className={sz} />,
	'📚': <Book className={sz} />,
	'🎵': <Music className={sz} />,
	'🎮': <Gamepad2 className={sz} />,
	'🚀': <Rocket className={sz} />,
	'⭐': <Star className={sz} />,
	'🌟': <Star className={sz} />,
	'❤': <Heart className={sz} />,
	'🔥': <Flame className={sz} />,
	'🏆': <Trophy className={sz} />,
	'👥': <Users className={sz} />,
	'☁': <Cloud className={sz} />,
	'💾': <Database className={sz} />,
	'🐛': <Bug className={sz} />,
	'🌐': <Globe className={sz} />,
	'📣': <Megaphone className={sz} />,
	'🛒': <ShoppingBag className={sz} />,
	'☕': <Coffee className={sz} />,
	'⚡': <Zap className={sz} />,
	'🧩': <LayoutGrid className={sz} />,
};

const VS = /\uFE0F/g;

function resolve(icon: string | undefined | null, level: number): ReactNode {
	if (icon) {
		const raw = icon.replace(VS, '').trim();
		const byEmoji = BY_EMOJI[raw];
		if (byEmoji) return byEmoji;
		const byName = BY_NAME[raw.toLowerCase()];
		if (byName) return byName;
	}
	if (level === 0) return <Folder className={sz} />;
	if (level === 1) return <FolderOpen className={sz} />;
	return <FileText className={sz} />;
}

export function SpaceIcon({ icon, level = 0, style }: { icon?: string | null; level?: number; style?: CSSProperties }) {
	return (
		<span className="inline-flex text-sm leading-none" style={style}>
			{resolve(icon, level)}
		</span>
	);
}
