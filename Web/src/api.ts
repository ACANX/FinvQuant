// 前端 API 调用封装：统一 /API/V1 前缀、错误处理、响应信封解析、超时控制
// 响应信封：{ code, message, data?, error? }（见 Docs/DevSpec/ApiSpec.md）

export interface ApiEnvelope<T = unknown> {
  code: number
  message: string
  data?: T
  error?: { code: string; catalog_version: string; retryable: boolean }
}

// 请求超时（毫秒）：超过则 AbortController 中断并提示（评审：原实现无超时/取消）
const REQUEST_TIMEOUT_MS = 30000

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

async function apiRequest<T>(path: string, init: RequestInit): Promise<T> {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS)
  let response: Response
  try {
    response = await fetch(`/API/V1${path}`, { ...init, signal: controller.signal })
  } catch (e) {
    if ((e as Error).name === 'AbortError') {
      throw new Error(`请求超时（${REQUEST_TIMEOUT_MS / 1000}s），请稍后重试`)
    }
    throw new Error('网络错误：无法连接服务端')
  } finally {
    clearTimeout(timer)
  }
  let envelope: ApiEnvelope<T>
  try {
    envelope = (await response.json()) as ApiEnvelope<T>
  } catch {
    throw new Error(`服务端响应异常（HTTP ${response.status}）`)
  }
  if (envelope.code !== 0) {
    throw new Error(envelope.message || '请求失败')
  }
  return envelope.data as T
}
