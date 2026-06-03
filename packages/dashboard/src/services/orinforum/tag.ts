import { request } from '@umijs/max';

/** 标签列表 POST /tags/list */
export async function list(params?: API.PageParams, options?: { [key: string]: any }) {
  return request<API.BaseResponse<API.Tag[]>>('/tags/list', {
    method: 'POST',
    data: params,
    ...(options || {}),
  });
}

/** 创建标签 POST /tags/create */
export async function create(name: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<API.Tag>>('/tags/create', {
    method: 'POST',
    data: { name },
    ...(options || {}),
  });
}

/** 更新标签 POST /tags/{id}/update */
export async function update(id: string, name: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<API.Tag>>(`/tags/${id}/update`, {
    method: 'POST',
    data: { name },
    ...(options || {}),
  });
}

/** 删除标签 POST /tags/{id}/delete */
export async function remove(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>(`/tags/${id}/delete`, {
    method: 'POST',
    ...(options || {}),
  });
}
