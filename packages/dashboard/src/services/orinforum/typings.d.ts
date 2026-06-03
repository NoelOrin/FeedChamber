// @ts-ignore
/* eslint-disable */

declare namespace API {
  /** OrinForum 统一响应 */
  interface BaseResponse<T = any> {
    code: number;
    msg: string;
    data: T;
    pagination?: { total: number } | null;
  }

  /** 分页参数 */
  interface PageParams {
    page?: number;
    pageSize?: number;
    startTime?: string;
    endTime?: string;
  }

  // ============ Auth ============
  interface LoginParams {
    username?: string;
    email?: string;
    password: string;
  }

  interface RegisterParams {
    username: string;
    email: string;
    password: string;
  }

  interface AuthResult {
    access_token: string;
    refresh_token: string;
    user: UserInfo;
  }

  // ============ User ============
  interface UserInfo {
    id: string;
    uuid: string;
    username: string;
    email: string;
    avatar: string;
    bio: string;
    role: string;
    created_at: string;
    updated_at: string;
  }

  interface UpdateProfileParams {
    username?: string;
    avatar?: string;
    bio?: string;
  }

  // ============ Post ============
  interface PostListItem {
    id: string;
    title: string;
    content_digest: string;
    author_id: string;
    author_name: string;
    author_avatar: string;
    category_id: string;
    category_name: string;
    status: string;
    pinned: boolean;
    locked: boolean;
    view_count: number;
    comment_count: number;
    like_count: number;
    favorite_count: number;
    created_at: string;
    updated_at: string;
  }

  interface PostDetail extends PostListItem {
    content?: string;
  }

  interface PostCreateParams {
    title: string;
    content: string;
    categoryId: string;
  }

  interface PostUpdateParams {
    title: string;
    content: string;
  }

  interface PostListParams extends PageParams {
    categoryName?: string;
    tagName?: string;
    authorId?: string;
    status?: string;
  }

  // ============ Category ============
  interface Category {
    id: string;
    name: string;
    description: string;
    created_at: string;
  }

  // ============ Tag ============
  interface Tag {
    id: string;
    name: string;
    created_at: string;
  }

  // ============ Content (Comment) ============
  interface ContentListItem {
    id: string;
    author_id: string;
    author_name: string;
    author_avatar: string;
    body: string;
    root_id: string;
    parent_id: string;
    like_count: number;
    created_at: string;
    updated_at: string;
  }

  // ============ Role ============
  interface Role {
    id: string;
    name: string;
    created_at: string;
  }

  // ============ Permission ============
  interface Permission {
    id: number;
    name: string;
    code: string;
    description: string;
    created_at: string;
  }

  // ============ Notification ============
  interface Notification {
    id: string;
    user_id: string;
    type: string;
    content: string;
    reference_id: string;
    sender_id: string;
    is_read: boolean;
    created_at: string;
  }

  // ============ Report ============
  interface Report {
    id: string;
    reporter_id: string;
    target_type: string;
    target_id: string;
    reason: string;
    status: string;
    created_at: string;
    updated_at: string;
  }

  // ============ Announcement ============
  interface Announcement {
    id: string;
    title: string;
    content: string;
    author_id: string;
    status: string;
    display_type: number;
    position: string;
    is_pinned: boolean;
    sort: number;
    expires_at: string | null;
    view_count: number;
    created_at: string;
    updated_at: string;
  }

  // ============ Admin ============
  interface AdminStats {
    totalUsers: number;
    totalPosts: number;
    totalComments: number;
    totalCategories: number;
    todayNewUsers: number;
    todayNewPosts: number;
    todayNewComments: number;
  }

  interface CurrentUser {
    name?: string;
    avatar?: string;
    userid?: string;
    email?: string;
    signature?: string;
    title?: string;
    group?: string;
    tags?: { key?: string; label?: string }[];
    notifyCount?: number;
    unreadCount?: number;
    country?: string;
    access?: string;
    geographic?: {
      province?: { label?: string; key?: string };
      city?: { label?: string; key?: string };
    };
    address?: string;
    phone?: string;
  }
}
