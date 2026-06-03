import { request } from '@umijs/max';

/** 创建举报 POST /reports/create */
export async function create(
  body: { targetType: string; targetId: string; reason: string },
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse<API.Report>>('/reports/create', {
    method: 'POST',
    data: body,
    ...(options || {}),
  });
}

/** 举报列表 POST /reports/list */
export async function list(
  params: API.PageParams & { status?: string },
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse<API.Report[]>>('/reports/list', {
    method: 'POST',
    data: params,
    ...(options || {}),
  });
}

/** 更新举报状态 POST /reports/{id}/status */
export async function updateStatus(id: string, status: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>(`/reports/${id}/status`, {
    method: 'POST',
    data: { status },
    ...(options || {}),
  });
}

/** 删除举报 POST /reports/{id}/delete */
export async function remove(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>(`/reports/${id}/delete`, {
    method: 'POST',
    ...(options || {}),
  });
}
