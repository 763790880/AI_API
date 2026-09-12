import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import DomesticPricingReference from '../DomesticPricingReference.vue'
import { appendMissingDomesticPricing, createDomesticPricingEntries, DOMESTIC_FX, DOMESTIC_MODELS } from '../domesticPricing'
import { createPricingFormEntry, formIntervalsToAPI, mTokToPerToken, perTokenToMTok } from '../types'

describe('国产模型价格隔离', () => {
  it('只包含指定六家各两个模型并保留官方来源', () => {
    const counts: Record<string, number> = {}
    for (const item of DOMESTIC_MODELS) {
      counts[item.vendor] = (counts[item.vendor] ?? 0) + 1
      expect(new URL(item.source).protocol).toBe('https:')
    }
    expect(counts).toEqual({ 阿里巴巴: 2, 月之暗面: 2, 智谱: 2, 字节跳动: 2, 腾讯: 2, DeepSeek: 2 })
    expect(new Set(DOMESTIC_MODELS.map(item => item.model)).size).toBe(12)
  })

  it('通过现有保存和回读转换后，百万 token 的金额仍等于人民币原价除汇率', () => {
    const entries = createDomesticPricingEntries(DOMESTIC_FX.cnyPerUSD, 'peak')
    entries.forEach((entry, i) => {
      const saved = mTokToPerToken(entry.input_price)!
      expect(saved * 1_000_000 * DOMESTIC_FX.cnyPerUSD).toBeCloseTo(DOMESTIC_MODELS[i].input, 7)
      expect(perTokenToMTok(saved)).toBeCloseTo(Number(entry.input_price), 8)
      expect(entry.time_ranges).toEqual([])
      expect(entry.long_context_pricing_enabled).toBeNull()
    })
  })

  it('保留千问输入长度阶梯，不猜测未公开缓存价或写入免费价', () => {
    const entries = createDomesticPricingEntries(7, 'peak')
    expect(entries[0].cache_read_price).toBeNull()
    expect(entries.every(item => item.cache_write_price === null)).toBe(true)
    const tiers = formIntervalsToAPI(entries[1].intervals)
    expect(tiers.map(tier => [tier.min_tokens, tier.max_tokens])).toEqual([[0, 256000], [256000, null]])
    expect(tiers[0].input_price! * 1_000_000 * 7).toBeCloseTo(2)
    expect(tiers[1].input_price! * 1_000_000 * 7).toBeCloseTo(6)
    expect(tiers[1].output_price! * 1_000_000 * 7).toBeCloseTo(24)
  })

  it('固定峰谷档位只影响新生成的 DeepSeek 模型', () => {
    const peak = createDomesticPricingEntries(7, 'peak')
    const offPeak = createDomesticPricingEntries(7, 'offPeak')
    expect(offPeak.slice(0, 10)).toEqual(peak.slice(0, 10))
    for (const i of [10, 11]) {
      expect(Number(offPeak[i].input_price)).toBe(Number(peak[i].input_price) / 2)
      expect(Number(offPeak[i].output_price)).toBe(Number(peak[i].output_price) / 2)
    }
  })

  it('补充不覆盖已有或通配符价格，重复导入和改汇率不改变已填金额', () => {
    const original = { ...createPricingFormEntry(), models: ['GLM-*'], input_price: 123, output_price: 456 }
    const existing = [original]
    appendMissingDomesticPricing(existing, 7, 'peak')
    expect(existing).toHaveLength(11)
    expect(existing[0]).toBe(original)
    expect(original.input_price).toBe(123)
    const snapshot = structuredClone(existing)
    appendMissingDomesticPricing(existing, 6, 'offPeak')
    expect(existing).toEqual(snapshot)
  })

  it.each([0, -1, NaN, Infinity])('拒绝无效汇率 %s', rate => {
    expect(() => createDomesticPricingEntries(rate, 'peak')).toThrow('换算汇率必须大于 0')
  })

  it('参考界面只有主动补充才发出事件，不能自动改已有价格', async () => {
    const wrapper = mount(DomesticPricingReference)
    expect(wrapper.emitted('append')).toBeUndefined()
    await wrapper.get('input').setValue(7)
    await wrapper.get('select').setValue('offPeak')
    expect(wrapper.emitted('append')).toBeUndefined()
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('append')).toEqual([[7, 'offPeak']])
    await wrapper.get('input').setValue(0)
    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('不修改已有价目')
  })
})
