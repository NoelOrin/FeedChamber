import { post } from '../client'
import type { AdminStats } from '../types'

export const adminService = {
  stats() {
    return post<AdminStats>('/admin/stats')
  },

  settings() {
    return post<Record<string, string>>('/admin/settings')
  },

  getSetting(key: string) {
    return post<string>(`/admin/settings/${key}`)
  },

  updateSetting(key: string, value: string) {
    return post<null>(`/admin/settings/${key}/update`, { value })
  },

  batchUpdateSettings(settings: Record<string, string>) {
    return post<null>('/admin/settings/batch', { settings })
  },

  deleteSetting(key: string) {
    return post<null>(`/admin/settings/${key}/delete`)
  },
}
