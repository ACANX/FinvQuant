// 前端登录会话状态：token 与用户信息持久化到 localStorage，刷新页面保持登录。
//
// 权威判定在服务端：本地校验只用于路由守卫的快速拦截（避免闪屏），
// 任何接口返回 401 时都会清空本地会话并跳回登录页。

import { reactive } from 'vue'

const TOKEN_KEY = 'finvquant.auth.token'
const USER_KEY = 'finvquant.auth.user'
const EXPIRES_KEY = 'finvquant.auth.expiresAt'

/** 登录用户信息（与 /Auth/Login、/Auth/Profile 返回结构一致） */
export interface AuthUser {
  uid: string
  nick_name: string
}

/** 会话状态 */
export interface AuthState {
  token: string
  user: AuthUser | null
  expiresAt: string | null
}

function readUser(): AuthUser | null {
  const raw = localStorage.getItem(USER_KEY)
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as AuthUser
    return parsed && typeof parsed.uid === 'string' ? parsed : null
  } catch {
    return null
  }
}

export const authState = reactive<AuthState>({
  token: localStorage.getItem(TOKEN_KEY) ?? '',
  user: readUser(),
  expiresAt: localStorage.getItem(EXPIRES_KEY),
})

/** 写入会话（登录成功后调用） */
export function setSession(token: string, user: AuthUser, expiresAt?: string): void {
  authState.token = token
  authState.user = user
  authState.expiresAt = expiresAt ?? null
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(USER_KEY, JSON.stringify(user))
  if (expiresAt) {
    localStorage.setItem(EXPIRES_KEY, expiresAt)
  } else {
    localStorage.removeItem(EXPIRES_KEY)
  }
}

/** 清空会话（登出或收到 401 时调用） */
export function clearSession(): void {
  authState.token = ''
  authState.user = null
  authState.expiresAt = null
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
  localStorage.removeItem(EXPIRES_KEY)
}

/** 是否已登录（本地快速判定） */
export function isLoggedIn(): boolean {
  if (!authState.token) return false
  if (authState.expiresAt) {
    const expireAt = Date.parse(authState.expiresAt)
    if (!Number.isNaN(expireAt) && expireAt <= Date.now()) {
      clearSession()
      return false
    }
  }
  return true
}

/** 当前令牌（供 api 层附加 Authorization 头） */
export function currentToken(): string {
  return authState.token
}

/** 会话失效事件名：api 层收到 401 时广播，路由层负责跳转 */
export const UNAUTHORIZED_EVENT = 'finvquant:unauthorized'
