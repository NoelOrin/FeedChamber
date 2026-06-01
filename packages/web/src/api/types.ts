export interface ApiResponse<T = unknown> {
  code: number
  msg: string
  data: T | null
}

export interface PaginatedResponse<T = unknown> {
  code: number
  msg: string
  data: T | null
  total: number
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
  created_at: string
  updated_at: string
}

export interface Post {
  id: string
  title: string
  content_id: string
  content_digest: string
  author_id: string
  category_id: string
  status: string
  pinned: boolean
  locked: boolean
  view_count: number
  comment_count: number
  like_count: number
  favorite_count: number
  deleted_at?: string
  created_at: string
  updated_at: string
}

export interface PostListItem {
  id: string
  title: string
  content_digest: string
  author_id: string
  author_name: string
  author_avatar: string
  category_id: string
  category_name: string
  status: string
  pinned: boolean
  locked: boolean
  view_count: number
  comment_count: number
  like_count: number
  favorite_count: number
  created_at: string
  updated_at: string
}

export interface Content {
  id: string
  author_id: string
  body: string
  root_id: string
  parent_id: string
  like_count: number
  deleted_at?: string
  created_at: string
  updated_at: string
}

export interface ContentListItem {
  id: string
  author_id: string
  author_name: string
  author_avatar: string
  body: string
  root_id: string
  parent_id: string
  like_count: number
  created_at: string
  updated_at: string
}

export interface Category {
  id: string
  name: string
  description: string
  created_at: string
}

export interface Tag {
  id: string
  name: string
  created_at: string
}

export interface Notification {
  id: string
  user_id: string
  type: string
  content: string
  reference_id: string
  sender_id: string
  is_read: boolean
  created_at: string
}

export interface Upload {
  id: string
  file_name: string
  stored_name: string
  path: string
  size: number
  mime_type: string
  uploader_id: string
  created_at: string
}

export interface Report {
  id: string
  reporter_id: string
  target_type: string
  target_id: string
  reason: string
  status: string
  created_at: string
  updated_at: string
}

export interface Role {
  id: string
  name: string
  created_at: string
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
