import { request } from '@umijs/max';

/** 权限列表 POST /permission/list */
export async function list(options?: { [key: string]: any }) {
  return request<API.BaseResponse<API.Permission[]>>('/permission/list', {
    method: 'POST',
    ...(options || {}),
  });
}

/** 获取角色权限 POST /permission/role */
export async function getByRole(role: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<{ role: string; permissions: string[] }>>('/permission/role', {
    method: 'POST',
    data: { role },
    ...(options || {}),
  });
}

/** 同步角色权限 POST /permission/sync */
export async function sync(role: string, permissions: string[], options?: { [key: string]: any }) {
  return request<API.BaseResponse<{ role: string; permissions: string[] }>>('/permission/sync', {
    method: 'POST',
    data: { role, permissions },
    ...(options || {}),
  });
}
