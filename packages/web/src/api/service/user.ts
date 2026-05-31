import { get, post } from '../client'
import type { User, ListParams } from '../types'

export const userService = {
  getProfile() {
    return get<User>('/user/profile')
  },

  getById(id: string) {
    return get<User>(`/user/${id}`)
  },

  list(params: ListParams) {
    return post<{ list: User[] }>('/user/list', params)
  },

  search(params: ListParams & { keyword: string; role?: string }) {
    return post<{ list: User[] }>('/user/search', params)
  },

  updateProfile(body: { username?: string; avatar?: string; bio?: string }) {
    return post<null>('/user/profile/update', body)
  },

  updateRole(id: string, role: string) {
    return post<null>(`/user/${id}/role/update`, { role })
  },

  delete(id: string) {
    return post<null>(`/user/${id}/delete`)
  },
}
