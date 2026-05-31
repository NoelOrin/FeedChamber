import { post } from '../client'
import type { Report, ListParams } from '../types'

export const reportService = {
  create(body: { targetType: string; targetId: string; reason: string }) {
    return post<Report>('/reports/create', body)
  },

  list(params: ListParams & { status?: string }) {
    return post<Report[]>('/reports/list', params)
  },

  updateStatus(id: string, status: string) {
    return post<null>(`/reports/${id}/status`, { status })
  },

  delete(id: string) {
    return post<null>(`/reports/${id}/delete`)
  },
}
