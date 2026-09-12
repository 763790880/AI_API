<template>
  <div class="mb-3 rounded-lg border border-gray-200 p-3 text-xs dark:border-dark-600">
    <p class="font-medium">国产模型官方价参考 · {{ DOMESTIC_PRICING_DATE }} · 六家厂商，各两个模型</p>
    <p class="mt-1 text-gray-500">下表为人民币 / 百万 token；导入后的价目为折算美元 / 百万 token，按现有规则保存和计费。</p>
    <div class="mt-3 flex flex-wrap items-end gap-3">
      <label>
        <span class="mb-1 block">换算汇率：1 美元 = 人民币</span>
        <input v-model.number="rate" type="number" min="0.000001" step="any" class="input w-36" aria-label="国产模型换算汇率">
      </label>
      <label>
        <span class="mb-1 block">DeepSeek 固定价格档位</span>
        <select v-model="deepseekRate" class="input" aria-label="DeepSeek 固定价格档位">
          <option value="peak">官方高峰价</option>
          <option value="offPeak">官方空闲价</option>
        </select>
      </label>
      <button type="button" class="btn btn-secondary" :disabled="!validRate" @click="emit('append', rate, deepseekRate)">补充缺少的官方模型与价格</button>
    </div>
    <p class="mt-2 text-gray-500">参考汇率日期 {{ DOMESTIC_FX.date }}（<a :href="DOMESTIC_FX.source" target="_blank" rel="noopener noreferrer" class="text-primary-600 underline">汇率来源</a>）。调整汇率仅用于本次补充，不修改已有价目；重新定价请先删除对应条目。</p>
    <p class="mt-2 text-amber-700 dark:text-amber-300">DeepSeek 官方高峰为北京时间周一至周五 09:00–12:00、14:00–18:00，其他时段含周末为空闲价。本模板使用选定的固定价，不自动切换峰谷。缓存存储等按时长收取的上游费用未纳入 token 价目。</p>
    <details class="mt-3">
      <summary class="cursor-pointer font-medium">查看 12 个模型的官方原价与来源</summary>
      <div class="mt-2 overflow-x-auto">
        <table class="w-full text-left">
          <thead><tr><th class="p-2">厂商 / 模型</th><th class="p-2">输入</th><th class="p-2">输出</th><th class="p-2">缓存命中</th></tr></thead>
          <tbody>
            <tr v-for="model in DOMESTIC_MODELS" :key="model.model" class="border-t border-gray-100 dark:border-dark-700">
              <td class="p-2"><a :href="model.source" target="_blank" rel="noopener noreferrer" class="text-primary-600 underline">{{ model.vendor }} · {{ model.model }}</a><p v-if="model.note" class="mt-1 max-w-lg text-gray-500">{{ model.note }}</p></td>
              <td class="p-2">￥{{ priceFor(model).input }}</td><td class="p-2">￥{{ priceFor(model).output }}</td><td class="p-2">{{ priceFor(model).cacheRead == null ? '未公开，不自动填入' : `￥${priceFor(model).cacheRead}` }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </details>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { DOMESTIC_FX, DOMESTIC_MODELS, DOMESTIC_PRICING_DATE, type DomesticModelPrice } from './domesticPricing'

const emit = defineEmits<{ (event: 'append', rate: number, deepseekRate: 'peak' | 'offPeak'): void }>()
const rate = ref<number>(DOMESTIC_FX.cnyPerUSD)
const deepseekRate = ref<'peak' | 'offPeak'>('peak')
const validRate = computed(() => Number.isFinite(rate.value) && rate.value > 0)
const priceFor = (model: DomesticModelPrice) => deepseekRate.value === 'offPeak' && model.offPeak ? model.offPeak : model
</script>
