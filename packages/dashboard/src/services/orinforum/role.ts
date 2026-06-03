import { request } from '@umijs/max';

/** 角色列表 POST /role/list */
export async function list(options?: { [key: string]: any }) {
  return request<API.BaseResponse<API.Role[]>>('/role/list', {
    method: 'POST',
    ...(options || {}),
  });
}

/** 创建角色 POST /role/create */
export async function create(name: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<API.Role>>('/role/create', {
    method: 'POST',
    data: { name },
    ...(options || {}),
  });
}

/** 删除角色 POST /role/{id}/delete */
export async function remove(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>(`/role/${id}/delete`, {
    method: 'POST',
    ...(options || {}),
  });
}
