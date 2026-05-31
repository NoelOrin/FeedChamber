import { post, postPaginated } from '../client'
import type { Category, ListParams } from '../types'

export const categoryService = {
  list(params?: ListParams) {
    return postPaginated<Category[]>('/categories/list', params)
  },

  create(body: { name: string; description?: string }) {
    return post<Category>('/categories/create', body)
  },

  update(id: string, body: { name: string; description?: string }) {
    return post<Category>(`/categories/${id}/update`, body)
  },

  delete(id: string) {
    return post<null>(`/categories/${id}/delete`)
  },
}
