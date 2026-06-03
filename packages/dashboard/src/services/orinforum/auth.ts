import { request } from '@umijs/max';

/** 注册 POST /auth/register */
export async function register(body: API.RegisterParams, options?: { [key: string]: any }) {
  return request<API.BaseResponse<API.AuthResult>>('/auth/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    data: body,
    ...(options || {}),
  });
}

/** 登录 POST /auth/login */
export async function login(body: API.LoginParams, options?: { [key: string]: any }) {
  return request<API.BaseResponse<API.AuthResult>>('/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    data: body,
    ...(options || {}),
  });
}

/** 登出 POST /auth/logout */
export async function logout(options?: { [key: string]: any }) {
  return request<API.BaseResponse<null>>('/auth/logout', {
    method: 'POST',
    ...(options || {}),
  });
}

/** 刷新令牌 POST /auth/refresh_token */
export async function refreshToken(refresh_token: string, options?: { [key: string]: any }) {
  return request<API.BaseResponse<{ access_token: string }>>('/auth/refresh_token', {
    method: 'POST',
    data: { refresh_token },
    ...(options || {}),
  });
}

/** 修改密码 POST /auth/change_password */
export async function changePassword(
  body: { old_password: string; new_password: string },
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse<null>>('/auth/change_password', {
    method: 'POST',
    data: body,
    ...(options || {}),
  });
}

/** 首次修改密码 POST /auth/first_change_password */
export async function firstChangePassword(
  body: { new_password: string; name?: string },
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse<null>>('/auth/first_change_password', {
    method: 'POST',
    data: body,
    ...(options || {}),
  });
}

/** 重置密码 POST /auth/reset_password */
export async function resetPassword(
  body: { username: string; new_password: string },
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse<null>>('/auth/reset_password', {
    method: 'POST',
    data: body,
    ...(options || {}),
  });
}
