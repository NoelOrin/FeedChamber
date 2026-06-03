import { get, post } from '../client'
import type { ListParams } from '../types'

export interface Announcement {
  id: string
  title: string
  content: string
  author_id: string
  status: string
  display_type: number // 1=toast 2=alert 3=modal
  position: string
  is_pinned: boolean
  sort: number
  expires_at: string | null
  view_count: number
  created_at: string
  updated_at: string
}

export const announcementService = {
  listPublished(params?: ListParams) {
    return post<{ list: Announcement[] }>('/announcements/published', params)
  },

  getByID(id: string) {
    return get<Announcement>(`/announcements/${id}`)
  },

  // admin
  list(params?: ListParams) {
    return post<{ list: Announcement[] }>('/admin/announcements/list', params)
  },

  create(data: Partial<Announcement>) {
    return post<Announcement>('/admin/announcements/create', data)
  },

  update(id: string, data: Partial<Announcement>) {
    return post<Announcement>(`/admin/announcements/${id}/update`, data)
  },

  delete(id: string) {
    return post<null>(`/admin/announcements/${id}/delete`)
  },

  publish(id: string) {
    return post<null>(`/admin/announcements/${id}/publish`)
  },

  unpublish(id: string) {
    return post<null>(`/admin/announcements/${id}/unpublish`)
  },
}
