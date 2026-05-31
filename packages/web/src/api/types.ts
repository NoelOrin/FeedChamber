export interface ApiResponse<T = unknown> {
  code: number
  msg: string
  data: T | null
}

export interface ListParams {
  page?: number
  pageSize?: number
  startTime?: string
  endTime?: string
}

export interface User {
  id: string
  uuid: string
  username: string
  email: string
  avatar: string
  bio: string
  role: string
  createdAt: string
  updatedAt: string
}

export interface Post {
  id: string
  title: string
  contentId: string
  contentDigest: string
  authorId: string
  categoryId: string
  status: string
  pinned: boolean
  locked: boolean
  viewCount: number
  commentCount: number
  likeCount: number
  favoriteCount: number
  deletedAt?: string
  createdAt: string
  updatedAt: string
}

export interface PostListItem {
  id: string
  title: string
  contentDigest: string
  authorId: string
  authorName: string
  authorAvatar: string
  categoryId: string
  categoryName: string
  status: string
  pinned: boolean
  locked: boolean
  viewCount: number
  commentCount: number
  likeCount: number
  favoriteCount: number
  createdAt: string
  updatedAt: string
}

export interface Content {
  id: string
  postId: string
  authorId: string
  body: string
  rootId: string
  parentId: string
  likeCount: number
  deletedAt?: string
  createdAt: string
  updatedAt: string
}

export interface ContentListItem {
  id: string
  postId: string
  authorId: string
  authorName: string
  authorAvatar: string
  body: string
  rootId: string
  parentId: string
  likeCount: number
  createdAt: string
  updatedAt: string
}

export interface Category {
  id: string
  name: string
  description: string
  createdAt: string
}

export interface Tag {
  id: string
  name: string
  createdAt: string
}

export interface Notification {
  id: string
  userId: string
  type: string
  content: string
  referenceId: string
  senderId: string
  isRead: boolean
  createdAt: string
}

export interface Upload {
  id: string
  fileName: string
  storedName: string
  path: string
  size: number
  mimeType: string
  uploaderId: string
  createdAt: string
}

export interface Report {
  id: string
  reporterId: string
  targetType: string
  targetId: string
  reason: string
  status: string
  createdAt: string
  updatedAt: string
}

export interface Role {
  id: string
  name: string
  createdAt: string
}

export interface AdminStats {
  totalUsers: number
  totalPosts: number
  totalComments: number
  totalCategories: number
  todayNewUsers: number
  todayNewPosts: number
  todayNewComments: number
}

export interface AuthResponse {
  access_token: string
  refresh_token: string
  user: User
}
