import {
	Book,
	Bug,
	Cloud,
	Code,
	Coffee,
	Cpu,
	Database,
	FileText,
	Flame,
	Folder,
	FolderOpen,
	Gamepad2,
	Globe,
	Hash,
	Heart,
	Laptop,
	Layers,
	type LucideIcon,
	Megaphone,
	MessageCircle,
	MessageSquare,
	Monitor,
	Music,
	Newspaper,
	Palette,
	Rocket,
	Server,
	Settings,
	Shield,
	ShoppingBag,
	Smartphone,
	Sparkles,
	Star,
	Tag,
	Terminal,
	Trophy,
	Users,
	Wrench,
	Zap,
} from '~/lib/icons';

/*
 * 版面图标：后端 `space.icon` 存图标名（laptop / server / message-circle …）。
 * 取值顺序：已知图标名（小写，允许 folder-open 这种写法）→ 历史 emoji 兼容 → Folder。
 *
 * 历史 emoji 分支只服务迁移前落库的数据，新数据一律写图标名；
 * 全部存量数据迁完后这段可以删掉。
 */
const BY_NAME: Record<string, LucideIcon> = {
	smartphone: Smartphone,
	phone: Smartphone,
	mobile: Smartphone,
	laptop: Laptop,
	computer: Laptop,
	monitor: Monitor,
	desktop: Monitor,
	settings: Settings,
	gear: Settings,
	message: MessageSquare,
	'message-circle': MessageCircle,
	'message-square': MessageSquare,
	chat: MessageSquare,
	comment: MessageCircle,
	talk: MessageCircle,
	server: Server,
	ops: Server,
	cloud: Cloud,
	database: Database,
	code: Code,
	terminal: Terminal,
	bug: Bug,
	book: Book,
	news: Newspaper,
	music: Music,
	game: Gamepad2,
	palette: Palette,
	rocket: Rocket,
	sparkles: Sparkles,
	star: Star,
	heart: Heart,
	flame: Flame,
	trophy: Trophy,
	shield: Shield,
	users: Users,
	tag: Tag,
	hash: Hash,
	layers: Layers,
	folder: Folder,
	'folder-open': FolderOpen,
	file: FileText,
	coffee: Coffee,
	wrench: Wrench,
	zap: Zap,
	globe: Globe,
	megaphone: Megaphone,
	shopping: ShoppingBag,
	cpu: Cpu,
};

/** 迁移前落库的 emoji，仅用于兼容存量数据。 */
const LEGACY_EMOJI: Record<string, LucideIcon> = {
	'📱': Smartphone,
	'💻': Laptop,
	'🖥': Monitor,
	'⚙': Settings,
	'🔧': Wrench,
	'💬': MessageCircle,
	'📁': Folder,
	'📂': FolderOpen,
	'📄': FileText,
	'📚': Book,
	'📰': Newspaper,
	'🎵': Music,
	'🎮': Gamepad2,
	'🎨': Palette,
	'🚀': Rocket,
	'✨': Sparkles,
	'⭐': Star,
	'❤': Heart,
	'🔥': Flame,
	'🏆': Trophy,
	'🛡': Shield,
	'👥': Users,
	'☁': Cloud,
	'💾': Database,
	'🐛': Bug,
	'🌐': Globe,
	'📣': Megaphone,
	'🛒': ShoppingBag,
	'☕': Coffee,
	'⚡': Zap,
	'🧩': Layers,
	'💡': Sparkles,
	'🛠': Wrench,
};

const VARIATION_SELECTOR = /\uFE0F/g;

function normalize(raw: string): string {
	return raw.replace(VARIATION_SELECTOR, '').trim().toLowerCase();
}

export function resolveSpaceIcon(raw?: string | null): LucideIcon {
	if (!raw) return Folder;
	const key = normalize(raw);
	return BY_NAME[key] ?? LEGACY_EMOJI[raw.replace(VARIATION_SELECTOR, '').trim()] ?? Folder;
}

export function SpaceIcon({ icon, size = 14, className }: { icon?: string | null; size?: number; className?: string }) {
	const Icon = resolveSpaceIcon(icon);
	return <Icon aria-hidden="true" size={size} className={className} />;
}
