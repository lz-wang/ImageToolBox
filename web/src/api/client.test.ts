import { afterEach, describe, expect, it, vi } from 'vitest'
import {
	ApiError,
	batchResize,
	batchWatermark,
	compressImage,
	fetchHealth,
	resizeImage,
} from './client'

function jsonResponse(status: number, body: unknown) {
	return new Response(JSON.stringify(body), { status })
}

function imageResponse(headers: Record<string, string> = {}) {
	return new Response(new Blob([new Uint8Array([1, 2, 3])]), {
		status: 200,
		headers,
	})
}

/** 捕获 fetch 收到的 FormData，便于断言序列化行为 */
function captureFetch(response: Response) {
	const calls: { url: string; init: RequestInit }[] = []
	vi.stubGlobal(
		'fetch',
		vi.fn(async (url: string | URL, init?: RequestInit) => {
			calls.push({ url: String(url), init: init ?? {} })
			return response
		}),
	)
	return calls
}

function submittedForm(calls: { url: string; init: RequestInit }[]): FormData {
	expect(calls.length).toBe(1)
	return calls[0].init.body as FormData
}

afterEach(() => {
	vi.unstubAllGlobals()
})

describe('fetchHealth', () => {
	it('解析健康状态 JSON', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => jsonResponse(200, { status: 'ok' })),
		)
		await expect(fetchHealth()).resolves.toEqual({ status: 'ok' })
	})

	it('错误响应抛出 ApiError 并携带后端错误消息', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => jsonResponse(500, { error: '内部错误' })),
		)
		const err = await fetchHealth().catch((e: unknown) => e)
		expect(err).toBeInstanceOf(ApiError)
		expect((err as ApiError).status).toBe(500)
		expect((err as ApiError).message).toBe('内部错误')
	})

	it('错误响应体不是 JSON 时使用默认消息', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => new Response('gateway timeout', { status: 504 })),
		)
		const err = await fetchHealth().catch((e: unknown) => e)
		expect(err).toBeInstanceOf(ApiError)
		expect((err as ApiError).message).toContain('504')
	})
})

describe('单图处理：options 序列化', () => {
	it('file 与 options 两个字段被正确序列化', async () => {
		const calls = captureFetch(
			imageResponse({
				'Content-Type': 'image/png',
				'Content-Disposition': 'attachment; filename="a_resized.png"',
				'X-ITB-Input-Size': '100',
				'X-ITB-Output-Size': '50',
			}),
		)
		const file = new File([new Uint8Array([1])], 'a.png', { type: 'image/png' })

		const result = await resizeImage(file, {
			width: 1200,
			height: 0,
			percent: '',
			mode: 'fit',
			anchor: 'center',
			filter: 'lanczos',
		})

		expect(calls[0].url).toBe('/api/v1/resize')
		const form = submittedForm(calls)
		expect(form.get('file')).toBeInstanceOf(File)
		expect(form.get('options')).toBe(
			JSON.stringify({
				width: 1200,
				height: 0,
				percent: '',
				mode: 'fit',
				anchor: 'center',
				filter: 'lanczos',
			}),
		)

		expect(result.filename).toBe('a_resized.png')
		expect(result.inputSize).toBe(100)
		expect(result.outputSize).toBe(50)
	})

	it('compress 只序列化 quality', async () => {
		const calls = captureFetch(imageResponse())
		await compressImage(new File([new Uint8Array([1])], 'a.png'), {
			quality: 80,
		})

		const form = submittedForm(calls)
		expect(calls[0].url).toBe('/api/v1/compress')
		expect(form.get('options')).toBe('{"quality":80}')
	})

	it('服务端错误转换为 ApiError', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => jsonResponse(400, { error: '调整尺寸失败' })),
		)
		const err = await resizeImage(new File([new Uint8Array([1])], 'a.png'), {
			width: 0,
			height: 0,
			percent: '',
			mode: 'fit',
			anchor: 'center',
			filter: 'lanczos',
		}).catch((e: unknown) => e)
		expect(err).toBeInstanceOf(ApiError)
		expect((err as ApiError).message).toBe('调整尺寸失败')
	})
})

describe('批处理：files[] 与附加文件序列化', () => {
	it('多文件放入 files 字段并解析统计头', async () => {
		const calls = captureFetch(
			new Response(new Blob([new Uint8Array([1])]), {
				status: 200,
				headers: {
					'Content-Disposition': 'attachment; filename=itb-batch-result.zip',
					'X-ITB-Success': '2',
					'X-ITB-Skipped': '1',
					'X-ITB-Failed': '0',
				},
			}),
		)
		const files = [
			new File([new Uint8Array([1])], 'a.png', { type: 'image/png' }),
			new File([new Uint8Array([2])], 'b.png', { type: 'image/png' }),
		]

		const result = await batchResize(files, {
			width: 100,
			height: 0,
			percent: '',
			mode: 'fit',
			anchor: 'center',
			filter: 'lanczos',
		})

		const form = submittedForm(calls)
		expect(form.getAll('files')).toHaveLength(2)
		expect(form.get('options')).toContain('"width":100')
		expect(result.success).toBe(2)
		expect(result.skipped).toBe(1)
		expect(result.failed).toBe(0)
		expect(result.filename).toBe('itb-batch-result.zip')
	})

	it('watermark 的附加文件单独携带', async () => {
		const calls = captureFetch(imageResponse())
		const wm = new File([new Uint8Array([3])], 'logo.png', {
			type: 'image/png',
		})
		const font = new File([new Uint8Array([4])], 'a.ttf')

		await batchWatermark(
			[new File([new Uint8Array([1])], 'a.png', { type: 'image/png' })],
			{
				type: 'image',
				text: '',
				mode: 'position',
				position: 'center',
				scale: 0.2,
			},
			{ watermark: wm, font },
		)

		const form = submittedForm(calls)
		expect(form.get('watermark')).toBeInstanceOf(File)
		expect(form.get('font')).toBeInstanceOf(File)
	})
})
