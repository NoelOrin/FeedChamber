import { post } from '../client'
import type { Category } from '../types'

export const categoryService = {
  list() {
    return post<Category[]>('/categories/list')
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
