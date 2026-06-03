import { request } from '@umijs/max';

/** 上传文件 POST /upload */
export async function upload(form: FormData, options?: { [key: string]: any }) {
  return request<API.BaseResponse<any>>('/upload', {
    method: 'POST',
    data: form,
    requestType: 'form',
    ...(options || {}),
  });
}

/** 获取预签名 POST /upload/presign */
export async function presign(
  body: { fileName: string; contentType?: string },
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse<{ uploadId: string; url: string; fields: Record<string, string>; fileUrl: string }>>(
    '/upload/presign',
    {
      method: 'POST',
      data: body,
      ...(options || {}),
    },
  );
}

/** 确认上传 POST /upload/confirm */
export async function confirm(body: { uploadId: string; key?: string }, options?: { [key: string]: any }) {
  return request<API.BaseResponse<any>>('/upload/confirm', {
    method: 'POST',
    data: body,
    ...(options || {}),
  });
}

/** 文件列表 POST /upload/list */
export async function list(params?: API.PageParams, options?: { [key: string]: any }) {
  return request<API.BaseResponse<any[]>>('/upload/list', {
    method: 'POST',
    data: params,
    ...(options || {}),
  });
}

/** 获取文件 GET /upload/{id} */
export async function getById(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<any>>(`/upload/${id}`, {
    method: 'GET',
    ...(options || {}),
  });
}

/** 删除文件 POST /upload/{id}/delete */
export async function remove(id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>(`/upload/${id}/delete`, {
    method: 'POST',
    ...(options || {}),
  });
}
