import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AdminUpstreamAccountModal from '../AdminUpstreamAccountModal.vue'
import { normalizeUpstreamBaseURL, upstreamModelErrorMessage } from '../upstreamAccount'

const mocks = vi.hoisted(() => ({ post: vi.fn(), create: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { post: mocks.post } }))
vi.mock('@/api/admin/accounts', () => ({ create: mocks.create }))

function setup() {
  return mount(AdminUpstreamAccountModal, {
    props: { show: true, groups: [] },
    global: { stubs: {
      BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
      GroupSelector: { props: ['modelValue', 'platform'], template: '<button data-test="group" :data-platform="platform" @click="$emit(\'update:modelValue\', [7])">group</button>' }
    } }
  })
}

beforeEach(() => { vi.resetAllMocks() })

describe('third-party upstream account', () => {
  it.each([
    [{ status: 502, message: 'Upstream model list request failed with HTTP 403' }, '上游拒绝了服务器请求'],
    [{ status: 502, message: 'Upstream model list request failed with HTTP 401' }, '上游 API Key'],
    [{ status: 502, message: 'Upstream model list request failed with HTTP 404' }, '模型列表接口'],
    [{ status: 401 }, '重新登录管理员'],
    [{ status: 403 }, '没有管理员权限'],
    [{ status: 0, code: 'ECONNABORTED' }, '读取模型超时'],
    [{ status: 0 }, '无法连接 CCAPI'],
    [{ status: 400, message: 'Invalid OpenAI base URL' }, '上游域名白名单'],
    [{ status: 502, message: 'Upstream returned no supported models' }, 'Key 的模型权限'],
    [{ status: 502, message: 'Upstream model list response was not valid JSON' }, '格式无法识别']
  ])('explains a known failure without exposing the response: %j', (error, expected) => {
    expect(upstreamModelErrorMessage(error)).toContain(expected)
  })
  it('does not display unexpected response text appended to a known error', () => {
    const message = upstreamModelErrorMessage({ status: 502, message: 'Upstream model list request failed with HTTP 403 secret-placeholder' })
    expect(message).not.toContain('secret-placeholder')
  })
  it('shows actionable URL validation before requesting models', async () => {
    const wrapper = setup()
    await wrapper.get('[name="upstream-url"]').setValue('https://relay.example.com/v1/chat/completions')
    await wrapper.get('[name="upstream-key"]').setValue('test-placeholder')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('不要填写具体请求接口')
    expect(mocks.post).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('shows the upstream HTTP status from the API client error', async () => {
    mocks.post.mockRejectedValue({ status: 502, message: 'Upstream model list request failed with HTTP 403' })
    const wrapper = setup()
    await wrapper.get('[name="upstream-url"]').setValue('relay.example.com')
    await wrapper.get('[name="upstream-key"]').setValue('test-placeholder')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('上游 /v1/models 返回 HTTP 403')
    await wrapper.get('form').trigger('submit')
    expect(mocks.create).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it.each(['relay.example.com', 'https://relay.example.com/v1/', 'https://relay.example.com/'])(
    'normalizes domain and v1 suffix: %s', value => {
      expect(normalizeUpstreamBaseURL(value)).toBe('https://relay.example.com')
    }
  )
  it.each(['', 'ftp://relay.example.com', 'https://user:secret@relay.example.com', 'https://relay.example.com?key=secret', 'https://relay.example.com/#secret', 'https://relay.example.com/v1/messages'])(
    'rejects invalid roots: %s', value => { expect(() => normalizeUpstreamBaseURL(value)).toThrow() }
  )
  it.each(['openai', 'anthropic'])('discovers models and creates a %s API key account', async platform => {
    mocks.post.mockResolvedValue({ data: { models: ['model-b', 'model-a', 'model-a'] } })
    mocks.create.mockResolvedValue({ id: 1 })
    const wrapper = setup()
    await wrapper.get('select').setValue(platform)
    await wrapper.get('[name="upstream-name"]').setValue('Relay')
    await wrapper.get('[name="upstream-url"]').setValue('https://relay.example.com/v1')
    await wrapper.get('[name="upstream-key"]').setValue('test-placeholder')
    await flushPromises()
    expect(mocks.post).toHaveBeenCalledWith('/admin/accounts/models/sync-upstream-preview', {
      platform, type: 'apikey', base_url: 'https://relay.example.com', api_key: 'test-placeholder'
    }, { timeout: 30000 })
    expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(2)
    await wrapper.get('[data-test="group"]').trigger('click')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.create).toHaveBeenCalledWith(expect.objectContaining({
      platform, type: 'apikey', group_ids: [7],
      credentials: { base_url: 'https://relay.example.com', api_key: 'test-placeholder', model_mapping: { 'model-a': 'model-a', 'model-b': 'model-b' } }
    }))
    expect(wrapper.emitted('created')).toHaveLength(1)
    wrapper.unmount()
  })
  it('does not filter groups by the upstream protocol', async () => {
    const wrapper = setup()
    expect(wrapper.get('[data-test="group"]').attributes('data-platform')).toBeUndefined()
    wrapper.unmount()
  })
  it('discards an old model response after credentials change', async () => {
    let resolve!: (value: unknown) => void
    mocks.post.mockReturnValue(new Promise(r => { resolve = r }))
    const wrapper = setup()
    await wrapper.get('[name="upstream-url"]').setValue('relay.example.com')
    await wrapper.get('[name="upstream-key"]').setValue('old-placeholder')
    await wrapper.get('[name="upstream-key"]').setValue('')
    resolve({ data: { models: ['old-model'] } })
    await flushPromises()
    expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(0)
    await wrapper.get('form').trigger('submit')
    expect(mocks.create).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('never displays raw upstream errors or creates after failed discovery', async () => {
    mocks.post.mockRejectedValue(new Error('sensitive-upstream-body'))
    const wrapper = setup()
    await wrapper.get('[name="upstream-url"]').setValue('relay.example.com')
    await wrapper.get('[name="upstream-key"]').setValue('test-placeholder')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('读取模型失败')
    expect(wrapper.text()).not.toContain('sensitive-upstream-body')
    await wrapper.get('form').trigger('submit')
    expect(mocks.create).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
