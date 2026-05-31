import { post, postPaginated } from '../client'
import type { Tag, ListParams } from '../types'

export const tagService = {
  list(params?: ListParams) {
    return postPaginated<Tag[]>('/tags/list', params)
  },

  create(name: string) {
    return post<Tag>('/tags/create', { name })
  },

  update(id: string, name: string) {
    return post<Tag>(`/tags/${id}/update`, { name })
  },

  delete(id: string) {
    return post<null>(`/tags/${id}/delete`)
  },
}
