import { request } from '@umijs/max';

/** 评论列表 POST /posts/{postId}/contents/list */
export async function list(
  postId: string,
  params: API.PageParams,
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse<API.ContentListItem[]>>(
    `/posts/${postId}/contents/list`,
    {
      method: 'POST',
      data: params,
      ...(options || {}),
    },
  );
}

/** 创建评论 POST /posts/{postId}/contents/create */
export async function create(
  postId: string,
  body: { body: string; parentId?: string },
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse<any>>(`/posts/${postId}/contents/create`, {
    method: 'POST',
    data: body,
    ...(options || {}),
  });
}

/** 更新评论 POST /posts/{postId}/contents/{id}/update */
export async function update(
  postId: string,
  id: string,
  body: { body: string },
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse<any>>(`/posts/${postId}/contents/${id}/update`, {
    method: 'POST',
    data: body,
    ...(options || {}),
  });
}

/** 删除评论 POST /posts/{postId}/contents/{id}/delete */
export async function remove(postId: string, id: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>(`/posts/${postId}/contents/${id}/delete`, {
    method: 'POST',
    ...(options || {}),
  });
}

/** 回复列表 POST /contents/{contentId}/replies */
export async function replies(
  contentId: string,
  params: API.PageParams,
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse<API.ContentListItem[]>>(`/contents/${contentId}/replies`, {
    method: 'POST',
    data: params,
    ...(options || {}),
  });
}
