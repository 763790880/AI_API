/** Store an API root so both model discovery and message forwarding append v1 once. */
export function normalizeUpstreamBaseURL(value: string): string {
  const raw = value.trim()
  if (!raw) throw new Error('请填写中转站域名')
  let url: URL
  try {
    url = new URL(raw.includes('://') ? raw : `https://${raw}`)
  } catch {
    throw new Error('中转站地址格式不正确，请填写域名或 API 根地址')
  }
  if (!['https:', 'http:'].includes(url.protocol) || !url.hostname ||
      url.username || url.password || url.search || url.hash) {
    throw new Error('请填写 HTTP/HTTPS 地址，不要包含用户名、密码、查询参数或锚点')
  }
  const path = url.pathname.replace(/\/+$/, '')
  if (/\/(chat\/completions|responses|messages|models)$/.test(path)) {
    throw new Error('请填写域名或 API 根地址，不要填写具体请求接口')
  }
  url.pathname = path.replace(/\/v1$/, '')
  return url.toString().replace(/\/+$/, '')
}

/** Translate only known diagnostics; never render arbitrary upstream response bodies. */
export function upstreamModelErrorMessage(error: unknown): string {
  const failure = error && typeof error === 'object'
    ? error as { status?: number; code?: string; message?: string }
    : {}
  if (failure.status === 401) return '登录已失效，请重新登录管理员账号后读取模型。'
  if (failure.status === 403) return '当前账号没有管理员权限，无法读取第三方中转站模型。'
  if (failure.code === 'ECONNABORTED' || failure.code === 'ETIMEDOUT') {
    return '读取模型超时，请检查 CCAPI 服务器到中转站的连接后重试。'
  }
  if (failure.status === 0) return '无法连接 CCAPI 服务，请检查网络后重试。'
  const message = typeof failure.message === 'string' ? failure.message : ''
  const upstreamStatus = /^Upstream model list request failed with HTTP (\d{3})$/.exec(message)?.[1]
  if (upstreamStatus) {
    const hints: Record<string, string> = {
      '401': '请检查上游 API Key 和接口协议。',
      '403': '上游拒绝了服务器请求，请检查 Key 权限及上游访问限制。',
      '404': '上游未提供此模型列表接口，请检查 Base URL。',
      '429': '上游请求过于频繁或额度受限，请稍后重试。'
    }
    return `读取模型失败：上游 /v1/models 返回 HTTP ${upstreamStatus}。${hints[upstreamStatus] || '请检查上游服务状态后重试。'}`
  }
  const known: Record<string, string> = {
    'Invalid OpenAI base URL': '中转站地址未通过服务器校验，请检查地址格式及系统设置中的上游域名白名单。',
    'Invalid Anthropic base URL': '中转站地址未通过服务器校验，请检查地址格式及系统设置中的上游域名白名单。',
    'Failed to request upstream model list': 'CCAPI 服务器无法连接上游模型列表接口，请检查上游连接、DNS 和证书。',
    'Failed to read upstream model list': '读取上游模型列表时连接中断，请重试。',
    'Upstream model list response was not valid JSON': '上游模型列表格式无法识别，请确认 Base URL 指向兼容 API，而非网页。',
    'Upstream model list response is too large': '上游模型列表响应过大，超过服务器读取限制。',
    'Upstream returned no supported models': '中转站没有返回可用模型，请检查 Key 的模型权限。'
  }
  return known[message] || '读取模型失败，请检查域名、Key 和中转站的 /v1/models 接口，然后重试。'
}
