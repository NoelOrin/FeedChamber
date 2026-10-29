import { TagList } from './tag-list';

/*
 * /timeline/tag —— 标签页（URL 契约不变）。
 *
 * 标签集是外壳级数据：根 layout 已把 qk.tags.list({page:1,page_size:50}) 写进每请求缓存并
 * 随 Providers 一起水合，客户端 TagList 的 useQuery 在服务端渲染阶段就能命中同一份数据，
 * 所以这里不需要再预取一次（再 dehydrate 一遍只会把同一批标签重复塞进 payload）。
 */
export default function TimelineTagPage() {
	return <TagList />;
}
