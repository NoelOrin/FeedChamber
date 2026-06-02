import { get, post, postForm } from '../client'
import type { Upload, UploadPresignData, ListParams } from '../types'

export const uploadService = {
  // 服务端代理上传（本地存储模式）
  upload(file: File) {
    const form = new FormData()
    form.append('file', file)
    return postForm<{ id: string; url: string; fileName: string; size: number }>('/upload/', form)
  },

  // 获取预签名 POST 信息
  presign(fileName: string, contentType: string) {
    return post<UploadPresignData>('/upload/presign', { fileName, contentType })
  },

  // 直传到对象存储（S3 / MinIO / R2 等）
  async directUpload(file: File): Promise<{ id: string; url: string; fileName: string; size: number }> {
    // 1. 获取预签名信息
    const presign = await this.presign(file.name, file.type)

    // 2. 构建 FormData，将 presigned fields 填入，最后附加文件
    const form = new FormData()
    for (const [key, value] of Object.entries(presign.fields)) {
      form.append(key, value)
    }
    form.append('file', file)

    // 3. POST 直传到对象存储
    const resp = await fetch(presign.url, { method: 'POST', body: form })
    if (!resp.ok) {
      throw new Error(`direct upload failed: ${resp.status}`)
    }

    // 4. 回调服务端确认上传完成
    return post<{ id: string; url: string; fileName: string; size: number }>('/upload/confirm', {
      uploadId: presign.uploadId,
    })
  },

  getUrl(id: string): string {
    return `/api/v1/upload/${id}`
  },

  list(params: ListParams) {
    return post<Upload[]>('/upload/list', params)
  },

  delete(id: string) {
    return post<null>(`/upload/${id}/delete`)
  },
}
