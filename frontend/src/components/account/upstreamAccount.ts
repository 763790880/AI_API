/** Store an API root so both model discovery and message forwarding append v1 once. */
export function normalizeUpstreamBaseURL(value: string): string {
  const raw = value.trim()
  if (!raw) throw new Error('请填写中转站域名')
  const url = new URL(raw.includes('://') ? raw : `https://${raw}`)
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
