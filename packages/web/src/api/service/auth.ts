import { get, post } from '../client'
import type { AuthResponse, User } from '../types'

export const authService = {
  register(body: { username: string; email: string; password: string }) {
    return post<AuthResponse>('/auth/register', body)
  },

  login(body: { username?: string; email?: string; password: string }) {
    return post<AuthResponse>('/auth/login', body)
  },

  logout() {
    return post<null>('/auth/logout')
  },

  refresh() {
    return post<{ access_token: string }>('/auth/refresh')
  },

  refreshToken(refresh_token: string) {
    return post<{ access_token: string }>('/auth/refresh_token', { refresh_token })
  },

  changePassword(body: { old_password: string; new_password: string }) {
    return post<null>('/auth/change_password', body)
  },

  firstChangePassword(body: { new_password: string; name?: string }) {
    return post<null>('/auth/first_change_password', body)
  },

  resetPassword(body: { username: string; new_password: string }) {
    return post<null>('/auth/reset_password', body)
  },
}
