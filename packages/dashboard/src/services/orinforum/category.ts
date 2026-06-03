import { request } from '@umijs/max';

/** 分类列表 POST /categories/list */
export async function list(params?: API.PageParams, options?: { [key: string]: any }) {
  return request<API.BaseResponse<API.Category[]>>('/categories/list', {
    method: 'POST',
    data: params,
    ...(options || {}),
  });
}

/** 创建分类 POST /categories/create */
export async function create(
  body: { name: string; description?: string },
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse<API.Category>>('/categories/create', {
    method: 'POST',
    data: body,
    ...(options || {}),
  });
}

/** 更新分类 POST /categories/{id}/update */
export async function update(
  id: string,
  body: { name: string; description?: string },
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse<API.Category>>(`/categories/${id}/update`, {
    method: 'POST',
    data: body,
    ...(options || {}),
  });
}

/** 删除分类 POST /categories/{id}/delete */
export async function remove(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>(`/categories/${id}/delete`, {
    method: 'POST',
    ...(options || {}),
  });
}
