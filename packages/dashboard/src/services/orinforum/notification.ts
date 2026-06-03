import { request } from '@umijs/max';

/** 通知列表 POST /notifications/list */
export async function list(params: API.PageParams, options?: { [key: string]: any }) {
  return request<API.BaseResponse<{ list: API.Notification[] }>>('/notifications/list', {
    method: 'POST',
    data: params,
    ...(options || {}),
  });
}

/** 未读数 POST /notifications/unread_count */
export async function unreadCount(options?: { [key: string]: any }) {
  return request<API.BaseResponse<{ count: number }>>('/notifications/unread_count', {
    method: 'POST',
    ...(options || {}),
  });
}

/** 标记已读 POST /notifications/{id}/read */
export async function read(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>(`/notifications/${id}/read`, {
    method: 'POST',
    ...(options || {}),
  });
}

/** 全部标记已读 POST /notifications/read_all */
export async function readAll(options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>('/notifications/read_all', {
    method: 'POST',
    ...(options || {}),
  });
}

/** 删除通知 POST /notifications/{id}/delete */
export async function remove(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>(`/notifications/${id}/delete`, {
    method: 'POST',
    ...(options || {}),
  });
}
