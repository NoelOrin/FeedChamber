import { post } from '../client'

export const settingService = {
  getPublic() {
    return post<Record<string, string>>('/settings/public')
  },
}
