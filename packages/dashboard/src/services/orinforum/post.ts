import { request } from '@umijs/max';

/** 帖子列表 POST /posts/list */
export async function list(params: API.PostListParams, options?: { [key: string]: any }) {
  return request<API.BaseResponse<API.PostListItem[]>>('/posts/list', {
    method: 'POST',
    data: params,
    ...(options || {}),
  });
}

/** 搜索帖子 POST /posts/search */
export async function search(
  params: API.PageParams & { q: string },
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse<API.PostListItem[]>>('/posts/search', {
    method: 'POST',
    data: params,
    ...(options || {}),
  });
}

/** 获取帖子详情 GET /posts/{id} */
export async function getById(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<API.PostDetail>>(`/posts/${id}`, {
    method: 'GET',
    ...(options || {}),
  });
}

/** 创建帖子 POST /posts/create */
export async function create(body: API.PostCreateParams, options?: { [key: string]: any }) {
  return request<API.BaseResponse<any>>('/posts/create', {
    method: 'POST',
    data: body,
    ...(options || {}),
  });
}

/** 更新帖子 POST /posts/{id}/update */
export async function update(id: string, body: API.PostUpdateParams, options?: { [key: string]: any }) {
  return request<API.BaseResponse<any>>(`/posts/${id}/update`, {
    method: 'POST',
    data: body,
    ...(options || {}),
  });
}

/** 删除帖子 POST /posts/{id}/delete */
export async function remove(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>(`/posts/${id}/delete`, {
    method: 'POST',
    ...(options || {}),
  });
}

/** 置顶/取消置顶 POST /posts/{id}/pin */
export async function togglePin(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<any>>(`/posts/${id}/pin`, {
    method: 'POST',
    ...(options || {}),
  });
}

/** 锁定/解锁 POST /posts/{id}/lock */
export async function toggleLock(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<any>>(`/posts/${id}/lock`, {
    method: 'POST',
    ...(options || {}),
  });
}

/** 设置帖子标签 POST /posts/{postId}/tags/set */
export async function setTags(postId: string, tags: string[], options?: { [key: string]: any }) {
  return request<API.BaseResponse<API.Tag[]>>(`/posts/${postId}/tags/set`, {
    method: 'POST',
    data: { tags },
    ...(options || {}),
  });
}
