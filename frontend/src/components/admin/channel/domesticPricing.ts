import { createPricingFormEntry, findModelConflict, type PricingFormEntry } from './types'

// Manual, reviewed snapshot of primary vendor documentation. This catalog is
// only a form default: it never updates global billing data or saved channels.
export const DOMESTIC_PRICING_DATE = '2026-09-12'
export const DOMESTIC_FX = { cnyPerUSD: 6.7082, date: '2026-09-11', source: 'https://api.frankfurter.dev/v1/2026-09-11?base=USD&symbols=CNY' } as const

interface DomesticTier {
  min: number
  max: number | null
  input: number
  output: number
  cacheRead?: number
}
export interface DomesticModelPrice {
  vendor: string
  model: string
  input: number
  output: number
  cacheRead?: number
  source: string
  note?: string
  tiers?: DomesticTier[]
  offPeak?: { input: number; output: number; cacheRead: number }
}

export const DOMESTIC_MODELS: readonly DomesticModelPrice[] = [
  { vendor: '阿里巴巴', model: 'qwen3.8-max', input: 12, output: 36,
    source: 'https://help.aliyun.com/zh/model-studio/model-pricing',
    note: '北京地域标准价；缓存命中价官方仅在百炼控制台公布，未自动填入。' },
  { vendor: '阿里巴巴', model: 'qwen3.7-plus', input: 2, output: 8, cacheRead: 0.4,
    source: 'https://help.aliyun.com/zh/model-studio/model-pricing',
    note: '采用原价，不含限时八折；输入超过 256,000 token 后为 6 / 24 元，已带入阶梯。缓存按默认隐式缓存 20% 定价。',
    tiers: [{ min: 0, max: 256000, input: 2, output: 8, cacheRead: 0.4 }, { min: 256000, max: null, input: 6, output: 24, cacheRead: 1.2 }] },
  { vendor: '月之暗面', model: 'kimi-k3', input: 20, output: 100, cacheRead: 2,
    source: 'https://platform.moonshot.cn/docs/pricing/chat' },
  { vendor: '月之暗面', model: 'kimi-k2.7-code', input: 6.5, output: 27, cacheRead: 1.3,
    source: 'https://platform.moonshot.cn/docs/pricing/chat' },
  { vendor: '智谱', model: 'glm-5.3', input: 8, output: 28, cacheRead: 2,
    source: 'https://docs.bigmodel.cn/cn/guide/start/pricing' },
  { vendor: '智谱', model: 'glm-5.3-flash', input: 0.8, output: 2.8, cacheRead: 0.23,
    source: 'https://docs.bigmodel.cn/cn/guide/start/pricing' },
  { vendor: '字节跳动', model: 'doubao-seed-evolving', input: 6, output: 30, cacheRead: 1.2,
    source: 'https://www.volcengine.com/docs/82379/1544106/', note: '不含上游单独收取的缓存存储费。' },
  { vendor: '字节跳动', model: 'doubao-seed-2.1-pro', input: 6, output: 30, cacheRead: 1.2,
    source: 'https://www.volcengine.com/docs/82379/1544106/', note: '不含上游单独收取的缓存存储费。' },
  { vendor: '腾讯', model: 'hy4-preview', input: 6, output: 18, cacheRead: 0.3,
    source: 'https://cloud.tencent.com/document/product/1823/130055', note: 'TokenHub 广州地域；官方最新预览模型。' },
  { vendor: '腾讯', model: 'hy3', input: 1, output: 4, cacheRead: 0.25,
    source: 'https://cloud.tencent.com/document/product/1823/130055' },
  { vendor: 'DeepSeek', model: 'deepseek-flash', input: 2, output: 8, cacheRead: 0.04,
    source: 'https://api-docs.deepseek.com/zh-cn/quick_start/pricing',
    offPeak: { input: 1, output: 4, cacheRead: 0.02 }, note: '对应 DeepSeek-V4.1-Flash；采用固定档位，不自动切换峰谷。' },
  { vendor: 'DeepSeek', model: 'deepseek-v4-pro', input: 9, output: 27, cacheRead: 0.3,
    source: 'https://api-docs.deepseek.com/zh-cn/quick_start/pricing',
    offPeak: { input: 4.5, output: 13.5, cacheRead: 0.15 }, note: '对应 DeepSeek-V4-Pro-0813；采用固定档位，不自动切换峰谷。' },
]

export function createDomesticPricingEntries(cnyPerUSD: number, deepseekRate: 'peak' | 'offPeak'): PricingFormEntry[] {
  if (!Number.isFinite(cnyPerUSD) || cnyPerUSD <= 0) throw new Error('换算汇率必须大于 0')
  const usd = (value?: number) => value == null ? null : value / cnyPerUSD
  return DOMESTIC_MODELS.map(model => {
    const price = deepseekRate === 'offPeak' && model.offPeak ? model.offPeak : model
    return {
      ...createPricingFormEntry(),
      models: [model.model],
      input_price: usd(price.input),
      output_price: usd(price.output),
      cache_read_price: usd(price.cacheRead),
      intervals: (model.tiers ?? []).map((tier, index) => ({
        min_tokens: tier.min, max_tokens: tier.max, tier_label: '', sort_order: index,
        input_price: usd(tier.input), output_price: usd(tier.output),
        cache_write_price: null, cache_read_price: usd(tier.cacheRead), per_request_price: null,
      })),
    }
  })
}

export function appendMissingDomesticPricing(existing: PricingFormEntry[], rate: number, mode: 'peak' | 'offPeak'): void {
  for (const entry of createDomesticPricingEntries(rate, mode)) {
    // Preserve saved/draft prices, including wildcard rules that cover a model.
    if (!existing.some(pricing => findModelConflict([...pricing.models, ...entry.models]))) {
      existing.push(entry)
    }
  }
}
