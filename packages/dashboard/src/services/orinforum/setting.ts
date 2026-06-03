import { request } from '@umijs/max';

/** 获取公开设置 POST /settings/public */
export async function publicSettings(options?: { [key: string]: any }) {
  return request<API.BaseResponse<Record<string, string>>>('/settings/public', {
    method: 'POST',
    ...(options || {}),
  });
}
