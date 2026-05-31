import { get, post, postForm } from '../client'
import type { Upload, ListParams } from '../types'

export const uploadService = {
  upload(file: File) {
    const form = new FormData()
    form.append('file', file)
    return postForm<{ id: string; url: string; fileName: string; size: number }>('/upload/', form)
  },

  getUrl(id: string): string {
    return `/api/v1/upload/${id}`
  },

  list(params: ListParams) {
    return post<Upload[]>('/upload/list', params)
  },

  delete(id: string) {
    return post<null>(`/upload/${id}/delete`)
  },
}
