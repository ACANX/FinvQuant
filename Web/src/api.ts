// 前端 API 调用封装：统一 /API/V1 前缀、错误处理、响应信封解析、超时控制
// 响应信封：{ code, message, data?, error? }（见 Docs/DevSpec/ApiSpec.md）
//
// 认证：除 /Auth/Login 等公开接口外，均自动附加 Authorization: Bearer <token>；
// 收到 401 时清空本地会话并广播 UNAUTHORIZED_EVENT，由路由层跳转登录页。

import { UNAUTHORIZED_EVENT, clearSession, currentToken } from './auth'

export interface ApiEnvelope<T = unknown> {
  code: number
  message: string
  data?: T
  error?: { code: string; catalog_version: string; retryable: boolean }
}

// 请求超时（毫秒）：超过则 AbortController 中断并提示（评审：原实现无超时/取消）
const REQUEST_TIMEOUT_MS = 30000

// 成功业务码集合（与 Docs/DevSpec/ApiSpec.md 一致：0 / 1 / 200 / 202）
// 分钟行情 KLine API 使用 code=1，统一下沉到此处判断，避免各页面重复处理。
const SUCCESS_CODES = new Set([0, 1, 200, 202])

/** GET 请求并解析响应信封；业务码非 0 时抛出 Error(message) */
export async function apiGet<T>(path: string): Promise<T> {
  return apiRequest<T>(path, { method: 'GET' })
}

/** POST JSON 请求并解析响应信封；业务码非 0 时抛出 Error(message) */
export async function apiPost<T>(path: string, body: unknown): Promise<T> {
  return apiRequest<T>(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

/** PUT JSON 请求并解析响应信封；业务码非 0 时抛出 Error(message) */
export async function apiPut<T>(path: string, body: unknown): Promise<T> {
  return apiRequest<T>(path, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

/** DELETE 请求并解析响应信封；业务码非 0 时抛出 Error(message) */
export async function apiDelete<T>(path: string): Promise<T> {
  return apiRequest<T>(path, { method: 'DELETE' })
}

async function apiRequest<T>(path: string, init: RequestInit): Promise<T> {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS)
  // 附加登录令牌（公开接口没有 token 时为空，不发送该头）
  const token = currentToken()
  const headers = new Headers(init.headers)
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  let response: Response
  try {
    response = await fetch(`/API/V1${path}`, { ...init, headers, signal: controller.signal })
  } catch (e) {
    if ((e as Error).name === 'AbortError') {
      throw new Error(`请求超时（${REQUEST_TIMEOUT_MS / 1000}s），请稍后重试`)
    }
    throw new Error('网络错误：无法连接服务端')
  } finally {
    clearTimeout(timer)
  }
  // 401：会话失效，清空本地会话并通知路由层跳转登录页。
  //
  // 例外：登录接口自身的 401 表示「用户名或密码错误」，必须把服务端消息
  // 原样抛给用户；若也走会话失效分支，用户会看到「未登录或登录已过期」，
  // 完全误导（而且会误清一次本来就不存在的会话）。
  const isLoginRequest = path.endsWith('/Meta/Finv/Auth/Login')
  if (response.status === 401 && !isLoginRequest) {
    clearSession()
    window.dispatchEvent(new CustomEvent(UNAUTHORIZED_EVENT))
    throw new Error('未登录或登录已过期，请重新登录')
  }
  let envelope: ApiEnvelope<T>
  try {
    envelope = (await response.json()) as ApiEnvelope<T>
  } catch {
    throw new Error(`服务端响应异常（HTTP ${response.status}）`)
  }
  if (!SUCCESS_CODES.has(envelope.code)) {
    throw new Error(envelope.message || '请求失败')
  }
  return envelope.data as T
}
