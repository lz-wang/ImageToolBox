/** 统一的 WebUI API 客户端：原生 fetch，无第三方依赖 */

export class ApiError extends Error {
	status: number

	constructor(message: string, status: number) {
		super(message)
		this.name = 'ApiError'
		this.status = status
	}
}

async function requestJSON<T>(path: string, init?: RequestInit): Promise<T> {
	const response = await fetch(path, init)
	if (!response.ok) {
		let message = `请求失败 (HTTP ${response.status})`
		try {
			const body = (await response.json()) as { error?: string }
			if (body?.error) {
				message = body.error
			}
		} catch {
			// 错误响应体不是 JSON 时使用默认消息
		}
		throw new ApiError(message, response.status)
	}
	return (await response.json()) as T
}

export interface HealthStatus {
	status: string
}

export function fetchHealth(): Promise<HealthStatus> {
	return requestJSON<HealthStatus>('/api/v1/health')
}
