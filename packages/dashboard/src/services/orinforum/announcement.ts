import { request } from '@umijs/max';

/** 已发布公告列表 POST /announcements/published */
export async function listPublished(params?: API.PageParams, options?: { [key: string]: any }) {
  return request<API.BaseResponse<{ list: API.Announcement[] }>>('/announcements/published', {
    method: 'POST',
    data: params,
    ...(options || {}),
  });
}

/** 公告详情 GET /announcements/{id} */
export async function getById(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<API.Announcement>>(`/announcements/${id}`, {
    method: 'GET',
    ...(options || {}),
  });
}

/** 管理端公告列表 POST /admin/announcements/list */
export async function adminList(params?: API.PageParams, options?: { [key: string]: any }) {
  return request<API.BaseResponse<{ list: API.Announcement[] }>>('/admin/announcements/list', {
    method: 'POST',
    data: params,
    ...(options || {}),
  });
}

/** 创建公告 POST /admin/announcements/create */
export async function create(data: Partial<API.Announcement>, options?: { [key: string]: any }) {
  return request<API.BaseResponse<API.Announcement>>('/admin/announcements/create', {
    method: 'POST',
    data,
    ...(options || {}),
  });
}

/** 更新公告 POST /admin/announcements/{id}/update */
export async function update(id: string, data: Partial<API.Announcement>, options?: { [key: string]: any }) {
  return request<API.BaseResponse<API.Announcement>>(`/admin/announcements/${id}/update`, {
    method: 'POST',
    data,
    ...(options || {}),
  });
}

/** 删除公告 POST /admin/announcements/{id}/delete */
export async function remove(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>(`/admin/announcements/${id}/delete`, {
    method: 'POST',
    ...(options || {}),
  });
}

/** 发布公告 POST /admin/announcements/{id}/publish */
export async function publish(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>(`/admin/announcements/${id}/publish`, {
    method: 'POST',
    ...(options || {}),
  });
}

/** 撤回公告 POST /admin/announcements/{id}/unpublish */
export async function unpublish(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>(`/admin/announcements/${id}/unpublish`, {
    method: 'POST',
    ...(options || {}),
  });
}
