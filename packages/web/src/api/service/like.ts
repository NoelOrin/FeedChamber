import { post } from '../client'

export const likeService = {
  toggle(contentId: string) {
    return post<{ liked: boolean }>(`/contents/${contentId}/like`)
  },

  status(contentId: string) {
    return post<{ liked: boolean; count: number }>(`/contents/${contentId}/like/status`)
  },
}
