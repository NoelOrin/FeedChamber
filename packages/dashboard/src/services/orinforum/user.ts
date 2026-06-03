import { request } from '@umijs/max';

/** 获取当前用户资料 POST /user/profile */
export async function profile(options?: { [key: string]: any }) {
  return request<API.BaseResponse<API.UserInfo>>('/user/profile', {
    method: 'POST',
    ...(options || {}),
  });
}

/** 根据 ID 获取用户 POST /user/{id} */
export async function getById(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<API.UserInfo>>(`/user/${id}`, {
    method: 'POST',
    ...(options || {}),
  });
}

/** 用户列表 POST /user/list */
export async function list(params: API.PageParams, options?: { [key: string]: any }) {
  return request<API.BaseResponse<{ list: API.UserInfo[] }>>('/user/list', {
    method: 'POST',
    data: params,
    ...(options || {}),
  });
}

/** 搜索用户 POST /user/search */
export async function search(
  params: API.PageParams & { keyword: string; role?: string },
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse<{ list: API.UserInfo[] }>>('/user/search', {
    method: 'POST',
    data: params,
    ...(options || {}),
  });
}

/** 更新资料 POST /user/profile */
export async function updateProfile(body: API.UpdateProfileParams, options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>('/user/profile', {
    method: 'POST',
    data: body,
    ...(options || {}),
  });
}

/** 更新角色 POST /user/{id}/role/update */
export async function updateRole(id: string, role: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>(`/user/${id}/role/update`, {
    method: 'POST',
    data: { role },
    ...(options || {}),
  });
}

/** 删除用户 POST /user/{id}/delete */
export async function remove(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>(`/user/${id}/delete`, {
    method: 'POST',
    ...(options || {}),
  });
}

/** 禁言用户 POST /user/{id}/banned */
export async function banned(
  id: string,
  body: { duration: string; reason?: string },
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse<null>>(`/user/${id}/banned`, {
    method: 'POST',
    data: body,
    ...(options || {}),
  });
}

/** 解禁用户 POST /user/{id}/unbanned */
export async function unbanned(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>(`/user/${id}/unbanned`, {
    method: 'POST',
    ...(options || {}),
  });
}

/** 获取禁言状态 POST /user/{id}/banned/status */
export async function bannedStatus(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<{ banned: boolean; expires_at?: string }>>(`/user/${id}/banned/status`, {
    method: 'POST',
    ...(options || {}),
  });
}
