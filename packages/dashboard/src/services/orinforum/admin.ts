import { request } from '@umijs/max';

/** 系统统计 POST /admin/stats */
export async function stats(options?: { [key: string]: any }) {
  return request<API.BaseResponse<API.AdminStats>>('/admin/stats', {
    method: 'POST',
    ...(options || {}),
  });
}

/** 禁言记录列表 POST /admin/banned/list */
export async function bannedList(params?: API.PageParams, options?: { [key: string]: any }) {
  return request<API.BaseResponse<any[]>>('/admin/banned/list', {
    method: 'POST',
    data: params,
    ...(options || {}),
  });
}

/** 获取所有设置 POST /admin/settings */
export async function settings(options?: { [key: string]: any }) {
  return request<API.BaseResponse<Record<string, string>>>('/admin/settings', {
    method: 'POST',
    ...(options || {}),
  });
}

/** 获取单个设置 POST /admin/settings/{key} */
export async function getSetting(key: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<string>>(`/admin/settings/${key}`, {
    method: 'POST',
    ...(options || {}),
  });
}

/** 更新设置 POST /admin/settings/{key}/update */
export async function updateSetting(key: string, value: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>(`/admin/settings/${key}/update`, {
    method: 'POST',
    data: { value },
    ...(options || {}),
  });
}

/** 批量设置 POST /admin/settings/batch */
export async function batchUpdateSettings(
  settings: Record<string, string>,
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse<null>>('/admin/settings/batch', {
    method: 'POST',
    data: { settings },
    ...(options || {}),
  });
}

/** 删除设置 POST /admin/settings/{key}/delete */
export async function deleteSetting(key: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>(`/admin/settings/${key}/delete`, {
    method: 'POST',
    ...(options || {}),
  });
}
