import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, fetchHealth } from './client'

function mockFetch(status: number, body: unknown) {
	return vi.fn(async () => new Response(JSON.stringify(body), { status }))
}

afterEach(() => {
	vi.unstubAllGlobals()
})

describe('fetchHealth', () => {
	it('解析健康状态 JSON', async () => {
		vi.stubGlobal('fetch', mockFetch(200, { status: 'ok' }))
		await expect(fetchHealth()).resolves.toEqual({ status: 'ok' })
	})

	it('错误响应抛出 ApiError 并携带后端错误消息', async () => {
		vi.stubGlobal('fetch', mockFetch(500, { error: '内部错误' }))
		const err = await fetchHealth().catch((e: unknown) => e)
		expect(err).toBeInstanceOf(ApiError)
		expect((err as ApiError).status).toBe(500)
		expect((err as ApiError).message).toBe('内部错误')
	})
})
