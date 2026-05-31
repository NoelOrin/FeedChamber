import { post } from '../client'
import type { Tag } from '../types'

export const tagService = {
  list() {
    return post<Tag[]>('/tags/list')
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
