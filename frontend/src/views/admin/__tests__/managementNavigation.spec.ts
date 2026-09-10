import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import UsersView from '../UsersView.vue'
import GroupsView from '../GroupsView.vue'

const { listUsers, listGroups, definitions, showError } = vi.hoisted(() => ({
  listUsers: vi.fn(),
  listGroups: vi.fn(),
  definitions: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: { list: listUsers },
    groups: {
      list: listGroups,
      getAll: vi.fn().mockResolvedValue([]),
      getCapacitySummary: vi.fn().mockResolvedValue([]),
      getUsageSummary: vi.fn().mockResolvedValue([])
    },
    userAttributes: { listEnabledDefinitions: definitions }
  }
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError }) }))
vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({ fetch: vi.fn() })
}))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))

const options = () => ({ global: { plugins: [createPinia()] } })
const emptyPage = { items: [], total: 0, pages: 0 }

describe('management page navigation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.spyOn(console, 'info').mockImplementation(() => {})
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    localStorage.clear()
    definitions.mockResolvedValue([])
    listUsers.mockResolvedValue(emptyPage)
  })
  afterEach(() => {
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('does not resume user initialization after leaving while definitions load', async () => {
    let finish!: (value: unknown[]) => void
    definitions.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    const addListener = vi.spyOn(window, 'addEventListener')
    const oldPage = shallowMount(UsersView, options())
    oldPage.unmount()
    addListener.mockClear()
    finish([])
    await flushPromises()
    expect(listUsers).not.toHaveBeenCalled()
    expect(addListener.mock.calls.some(([event]) => event === 'scroll')).toBe(false)

    const newPage = shallowMount(UsersView, options())
    await flushPromises()
    expect(listUsers).toHaveBeenCalledTimes(1)
    newPage.unmount()
  })

  it('cancels the old group request and ignores a late network failure', async () => {
    let fail!: (reason: unknown) => void
    listGroups.mockReturnValueOnce(new Promise((_resolve, reject) => { fail = reject }))
    const page = shallowMount(GroupsView, options())
    const signal = listGroups.mock.calls[0][3].signal as AbortSignal
    expect(signal.aborted).toBe(false)
    page.unmount()
    expect(signal.aborted).toBe(true)
    fail({ message: 'Network error' })
    await flushPromises()
    expect(showError).not.toHaveBeenCalled()
  })

  it('still reports a group request failure while the page is active', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    listGroups.mockRejectedValueOnce({ message: 'private response text', code: 'ERR_NETWORK' })
    const page = shallowMount(GroupsView, options())
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('admin.groups.failedToLoad')
    expect(console.warn).toHaveBeenCalledWith(
      '[management-page]',
      expect.objectContaining({
        page: 'groups',
        event: 'load-error',
        request_seq: 1,
        error_code: 'ERR_NETWORK'
      })
    )
    expect(JSON.stringify(vi.mocked(console.warn).mock.calls)).not.toContain('private response text')
    page.unmount()
    expect(console.info).toHaveBeenCalledWith(
      '[management-page]',
      expect.objectContaining({ page: 'groups', event: 'unmount', request_active: false })
    )
  })

  it('clears a pending group search when leaving', async () => {
    vi.useFakeTimers()
    listGroups.mockReturnValue(new Promise(() => {}))
    const page = shallowMount(GroupsView, options())
    ;(page.vm as unknown as { handleSearch: () => void }).handleSearch()
    page.unmount()
    await vi.runAllTimersAsync()
    expect(listGroups).toHaveBeenCalledTimes(1)
  })

  it('ignores a late user failure after leaving even without a cancellation code', async () => {
    let fail!: (reason: unknown) => void
    listUsers.mockReturnValueOnce(new Promise((_resolve, reject) => { fail = reject }))
    const page = shallowMount(UsersView, options())
    await flushPromises()
    page.unmount()
    fail({ message: 'Network error' })
    await flushPromises()
    expect(showError).not.toHaveBeenCalled()
  })
})
