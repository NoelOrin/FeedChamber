import { post, postPaginated } from '../client'
import type { Content, ContentListItem, ListParams } from '../types'

export const contentService = {
  list(postId: string, params: ListParams) {
    return postPaginated<ContentListItem[]>(`/posts/${postId}/contents/list`, params)
  },

  create(postId: string, body: { body: string; parentId?: string }) {
    return post<Content>(`/posts/${postId}/contents/create`, body)
  },

  update(postId: string, id: string, body: { body: string }) {
    return post<Content>(`/posts/${postId}/contents/${id}/update`, body)
  },

  delete(postId: string, id: string) {
    return post<null>(`/posts/${postId}/contents/${id}/delete`)
  },

  replies(contentId: string, params: ListParams) {
    return postPaginated<ContentListItem[]>(`/contents/${contentId}/replies`, params)
  },
}
