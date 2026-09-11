import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AdminUpstreamAccountModal from '../AdminUpstreamAccountModal.vue'
import { normalizeUpstreamBaseURL } from '../upstreamAccount'

const mocks = vi.hoisted(() => ({ post: vi.fn(), create: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { post: mocks.post } }))
vi.mock('@/api/admin/accounts', () => ({ create: mocks.create }))

function setup() {
  return mount(AdminUpstreamAccountModal, {
    props: { show: true, groups: [] },
    global: { stubs: {
      BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
      GroupSelector: { props: ['modelValue'], template: '<button data-test="group" @click="$emit(\'update:modelValue\', [7])">group</button>' }
    } }
  })
}

beforeEach(() => { vi.resetAllMocks() })

describe('third-party upstream account', () => {
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
