import { post, postPaginated } from '../client';
import type { Conversation, ConversationListItem, ListParams, Message } from '../types';

export const pmService = {
	listConversations(params: ListParams) {
		return postPaginated<ConversationListItem[]>('/messages/conversations/list', params);
	},

	create(body: { peer_user_id: string; body: string }) {
		return post<{ conversation: Conversation; message: Message | null }>('/messages/conversations/create', body);
	},

	listMessages(id: string, params: ListParams) {
		return postPaginated<Message[]>(`/messages/conversations/${id}/messages/list`, params);
	},

	send(id: string, body: { body: string }) {
		return post<Message>(`/messages/conversations/${id}/messages/create`, body);
	},

	markRead(id: string) {
		return post<null>(`/messages/conversations/${id}/read`, {});
	},
};
