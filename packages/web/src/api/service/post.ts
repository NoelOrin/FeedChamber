import { get, post } from '../client'
import type { Post, PostListItem, Tag, ListParams } from '../types'

export interface PostListParams extends ListParams {
  categoryId?: string
  authorId?: string
  status?: string
}

export const postService = {
  list(params: PostListParams) {
    return post<PostListItem[]>('/posts/list', params)
  },

  search(params: ListParams & { q: string }) {
    return post<PostListItem[]>('/posts/search', params)
  },

  getById(id: string) {
    return get<PostListItem>(`/posts/${id}`)
  },

  create(body: { title: string; content: string; categoryId: string }) {
    return post<Post>('/posts/create', body)
  },

  update(id: string, body: { title: string; content: string }) {
    return post<Post>(`/posts/${id}/update`, body)
  },

  delete(id: string) {
    return post<null>(`/posts/${id}/delete`)
  },

  togglePin(id: string) {
    return post<Post>(`/posts/${id}/pin`)
  },

  toggleLock(id: string) {
    return post<Post>(`/posts/${id}/lock`)
  },

  getTags(postId: string) {
    return post<Tag[]>(`/posts/${postId}/tags`)
  },

  setTags(postId: string, tags: string[]) {
    return post<Tag[]>(`/posts/${postId}/tags/set`, { tags })
  },
}
