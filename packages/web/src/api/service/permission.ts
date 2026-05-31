import { post } from '../client'

export interface Permission {
  id: number
  name: string
  code: string
  description: string
  created_at: string
}

export const permissionService = {
  list() {
    return post<Permission[]>('/permission/list')
  },

  getByRole(role: string) {
    return post<{ role: string; permissions: string[] }>('/permission/role', { role })
  },

  sync(role: string, permissions: string[]) {
    return post<{ role: string; permissions: string[] }>('/permission/sync', { role, permissions })
  },
}
