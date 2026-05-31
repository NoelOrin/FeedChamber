import { post } from '../client'
import type { Role } from '../types'

export const roleService = {
  list() {
    return post<Role[]>('/role/list')
  },

  create(name: string) {
    return post<Role>('/role/create', { name })
  },

  delete(id: string) {
    return post<null>(`/role/${id}/delete`)
  },
}
