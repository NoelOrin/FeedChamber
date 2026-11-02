import { pmService } from '@feedchamber/api-client';
import { queryOptions } from '@tanstack/react-query';

import { qk } from '~/lib/query/keys';

/*
 * 私信取数契约：RSC 预取与客户端 useQuery 共用同一份 key/params，
 * 否则水合后首帧会立刻重发一次请求。尺寸照旧 loader（列表 50、消息流 100）。
 */
const CONVERSATION_PAGE_SIZE = 50;
const THREAD_PAGE_SIZE = 100;

const CONVERSATION_LIST_PARAMS = { page: 1, page_size: CONVERSATION_PAGE_SIZE };
/** 旧 $id loader 用会话列表（100 条）反查对端昵称/头像，平移期保持同一份缓存。 */
const MESSAGE_LIST_PARAMS = { page: 1, page_size: THREAD_PAGE_SIZE };

export function conversationsOptions(params = CONVERSATION_LIST_PARAMS) {
	return queryOptions({
		queryKey: qk.pm.conversations(params),
		queryFn: () => pmService.listConversations(params),
	});
}

export function messagesOptions(conversationId: string) {
	return queryOptions({
		queryKey: qk.pm.messages(conversationId, MESSAGE_LIST_PARAMS),
		queryFn: () => pmService.listMessages(conversationId, MESSAGE_LIST_PARAMS),
	});
}
