<template>
  <BaseDialog :show="show" title="添加第三方中转站" width="wide" :close-disabled="submitting" @close="emit('close')">
    <form id="admin-upstream-account" class="space-y-5" @submit.prevent="submit">
      <p class="input-hint">使用中转站的域名和 API Key 添加账号。模型自动读取，收费沿用所选分组和渠道的管理员定价。</p>
      <fieldset :disabled="submitting" class="space-y-5">
        <label class="block"><span class="input-label">账号名称</span>
          <input v-model="name" name="upstream-name" required class="input" placeholder="例如：备用中转站" />
        </label>
        <label class="block"><span class="input-label">接口协议</span>
          <select v-model="platform" name="upstream-protocol" class="input" @change="readModels">
            <option value="openai">OpenAI 兼容（Chat Completions / Responses）</option>
            <option value="anthropic">Anthropic 兼容（Messages）</option>
          </select>
        </label>
        <label class="block"><span class="input-label">中转站域名 / Base URL</span>
          <input v-model="baseURL" name="upstream-url" required class="input" placeholder="https://relay.example.com 或 https://relay.example.com/v1" @change="readModels" />
          <span class="input-hint">若服务器启用了上游白名单，请先在系统设置的“上游域名白名单”中添加此域名。</span>
        </label>
        <label class="block"><span class="input-label">API Key</span>
          <input v-model="apiKey" name="upstream-key" required type="password" autocomplete="new-password" class="input" placeholder="填写该中转站提供的 API Key" @change="readModels" />
        </label>
        <div>
          <div class="flex items-center justify-between gap-3">
            <span class="input-label">可用模型（已选 {{ selectedModels.length }} 个）</span>
            <button type="button" class="btn btn-secondary" :disabled="loading || !baseURL.trim() || !apiKey.trim()" @click="readModels">{{ loading ? '正在读取…' : '重新读取模型' }}</button>
          </div>
          <p v-if="!models.length" class="input-hint">填写域名和 Key 后自动读取模型。读取失败时请检查地址、Key 及上游的模型列表接口。</p>
          <div v-else class="grid max-h-56 grid-cols-1 gap-2 overflow-auto rounded-lg border p-3 sm:grid-cols-2 dark:border-dark-600">
            <label v-for="model in models" :key="model" class="flex items-center gap-2 break-all text-sm">
              <input v-model="selectedModels" type="checkbox" :value="model" />{{ model }}
            </label>
          </div>
        </div>
        <GroupSelector v-model="groupIDs" :groups="groups" :platform="platform" label="投放分组" />
        <p class="input-hint">至少选择一个相同协议的分组，使账号可以参与调度。上游模型必须兼容所选协议；新增模型的售价请在渠道中配置。</p>
        <label class="block"><span class="input-label">最大并发</span>
          <input v-model.number="concurrency" name="upstream-concurrency" type="number" min="1" max="1000" required class="input" />
        </label>
      </fieldset>
      <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
    </form>
    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="submitting" @click="emit('close')">取消</button>
      <button type="submit" form="admin-upstream-account" class="btn btn-primary" :disabled="submitting || loading || !selectedModels.length || !groupIDs.length">{{ submitting ? '正在创建…' : '创建账号' }}</button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import GroupSelector from '@/components/common/GroupSelector.vue'
import { apiClient } from '@/api/client'
import { create } from '@/api/admin/accounts'
import { normalizeUpstreamBaseURL } from './upstreamAccount'
import type { AdminGroup } from '@/types'

const props = defineProps<{ show: boolean; groups: AdminGroup[] }>()
const emit = defineEmits<{ close: []; created: [] }>()
const name = ref('')
const platform = ref<'openai' | 'anthropic'>('openai')
const baseURL = ref('')
const apiKey = ref('')
const models = ref<string[]>([])
const selectedModels = ref<string[]>([])
const groupIDs = ref<number[]>([])
const concurrency = ref(5)
const loading = ref(false)
const submitting = ref(false)
const error = ref('')
let generation = 0

watch([platform, baseURL, apiKey], () => {
  generation++
  models.value = []
  selectedModels.value = []
  loading.value = false
  error.value = ''
}, { flush: 'sync' })
watch(platform, () => { groupIDs.value = [] }, { flush: 'sync' })
watch(() => props.show, () => {
  generation++
  name.value = ''
  baseURL.value = ''
  apiKey.value = ''
  models.value = []
  selectedModels.value = []
  groupIDs.value = []
  error.value = ''
  loading.value = false
  concurrency.value = 5
})

async function readModels() {
  if (!props.show || submitting.value || !baseURL.value.trim() || !apiKey.value.trim()) return
  const current = ++generation
  models.value = []
  selectedModels.value = []
  error.value = ''
  try {
    const normalized = normalizeUpstreamBaseURL(baseURL.value)
    loading.value = true
    const { data } = await apiClient.post<{ models: string[] }>('/admin/accounts/models/sync-upstream-preview', {
      platform: platform.value, type: 'apikey', base_url: normalized, api_key: apiKey.value.trim()
    }, { timeout: 30000 })
    if (current !== generation || !props.show) return
    models.value = [...new Set(data.models.map(model => model.trim()).filter(Boolean))].sort()
    selectedModels.value = [...models.value]
    if (!models.value.length) error.value = '中转站没有返回可用模型，请检查 Key 的模型权限。'
  } catch {
    if (current === generation) error.value = '读取模型失败，请检查域名、Key 和中转站的 /v1/models 接口，然后重试。'
  } finally {
    if (current === generation) loading.value = false
  }
}

async function submit() {
  if (submitting.value || loading.value || !selectedModels.value.length || !groupIDs.value.length) return
  if (!name.value.trim() || !apiKey.value.trim() || !Number.isInteger(concurrency.value) || concurrency.value < 1 || concurrency.value > 1000) return
  submitting.value = true
  error.value = ''
  try {
    await create({
      name: name.value.trim(), platform: platform.value, type: 'apikey',
      credentials: {
        base_url: normalizeUpstreamBaseURL(baseURL.value), api_key: apiKey.value.trim(),
        model_mapping: Object.fromEntries(selectedModels.value.map(model => [model, model]))
      },
      extra: { account_source: 'third_party' },
      concurrency: concurrency.value, priority: 1, group_ids: [...groupIDs.value]
    })
    emit('created')
    emit('close')
  } catch {
    error.value = '创建失败，请检查分组绑定是否允许当前账号，或查看管理员错误记录后重试。'
  } finally {
    submitting.value = false
  }
}
</script>
