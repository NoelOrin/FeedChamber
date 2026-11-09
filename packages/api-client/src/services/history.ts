import { post, postPaginated } from '../client';
import type { PostListItem } from '../types';

/**
 * 浏览记录（`POST /history/*`）。
 *
 * 三条都是 POST 且带动词后缀（api-style 硬约束）；分页字段是规范的
 * `page` / `pageSize`，不是旧的 `ListParams.page_size`。
 *
 * 访问者身份由后端从会话取，这里没有任何 user id 参数——越权写口不开。
 * 返回的帖子条目天然只含「访问者现在还看得见」的那些（可见性谓词在 SQL 里），
 * 所以列表项可能少于历史上真正读过的数量，这是隐私与一致性优先的结果。
 */
export const historyService = {
	list(params: { page?: number; pageSize?: number } = {}) {
		return postPaginated<PostListItem[]>('/history/list', params);
	},

	/** 删除某一篇的浏览记录。后端幂等：本就没有这条也返回成功。 */
	delete(postId: string) {
		return post<null>(`/history/${postId}/delete`, {});
	},

	/** 清空当前用户的全部浏览记录。 */
	clear() {
		return post<null>('/history/clear', {});
	},
};
