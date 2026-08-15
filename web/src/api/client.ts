/** 统一的 WebUI API 客户端：原生 fetch，无第三方依赖 */

export class ApiError extends Error {
	status: number

	constructor(message: string, status: number) {
		super(message)
		this.name = 'ApiError'
		this.status = status
	}
}

async function parseErrorMessage(response: Response): Promise<string> {
	let message = `请求失败 (HTTP ${response.status})`
	try {
		const body = (await response.json()) as { error?: string }
		if (body?.error) {
			message = body.error
		}
	} catch {
		// 错误响应体不是 JSON 时使用默认消息
	}
	return message
}

async function requestJSON<T>(path: string, init?: RequestInit): Promise<T> {
	const response = await fetch(path, init)
	if (!response.ok) {
		throw new ApiError(await parseErrorMessage(response), response.status)
	}
	return (await response.json()) as T
}

export interface HealthStatus {
	status: string
}

export function fetchHealth(): Promise<HealthStatus> {
	return requestJSON<HealthStatus>('/api/v1/health')
}

// ---------- 单图处理 ----------

export interface CompressOptions {
	quality: number
}

export interface ResizeOptions {
	width: number
	height: number
	percent: string
	mode: string
	anchor: string
	filter: string
}

export interface CropOptions {
	anchor: string
	width: string
	height: string
}

export interface ConvertOptions {
	to: string
	quality: number
	lossless: boolean
	background: string
}

export interface WatermarkOptions {
	type: string
	text: string
	mode: string
	position: string
	opacity?: number
	color?: string
	fontSize?: number
	space?: number
	angle?: number
	margin?: number
	scale?: number
}

export interface ProcessResult {
	blob: Blob
	url: string
	contentType: string
	filename: string
	inputSize: number
	outputSize: number
}

async function processImage(
	path: string,
	file: File,
	options: unknown,
	extra?: Record<string, File>,
): Promise<ProcessResult> {
	const form = new FormData()
	form.append('file', file)
	form.append('options', JSON.stringify(options ?? {}))
	for (const [field, extraFile] of Object.entries(extra ?? {})) {
		form.append(field, extraFile)
	}

	const response = await fetch(path, { method: 'POST', body: form })
	if (!response.ok) {
		throw new ApiError(await parseErrorMessage(response), response.status)
	}
	const blob = await response.blob()
	const disposition = response.headers.get('Content-Disposition') ?? ''
	const match = /filename="([^"]+)"/.exec(disposition)
	return {
		blob,
		url: URL.createObjectURL(blob),
		contentType: response.headers.get('Content-Type') ?? blob.type,
		filename: match?.[1] ?? 'result',
		inputSize: Number(response.headers.get('X-ITB-Input-Size') ?? 0),
		outputSize: Number(response.headers.get('X-ITB-Output-Size') ?? blob.size),
	}
}

export function compressImage(
	file: File,
	options: CompressOptions,
): Promise<ProcessResult> {
	return processImage('/api/v1/compress', file, options)
}

export function resizeImage(
	file: File,
	options: ResizeOptions,
): Promise<ProcessResult> {
	return processImage('/api/v1/resize', file, options)
}

export function cropImage(
	file: File,
	options: CropOptions,
): Promise<ProcessResult> {
	return processImage('/api/v1/crop', file, options)
}

export function convertImage(
	file: File,
	options: ConvertOptions,
): Promise<ProcessResult> {
	return processImage('/api/v1/convert', file, options)
}

export function watermarkImage(
	file: File,
	options: WatermarkOptions,
	extra: { watermark?: File; font?: File } = {},
): Promise<ProcessResult> {
	const files: Record<string, File> = {}
	if (extra.watermark) {
		files.watermark = extra.watermark
	}
	if (extra.font) {
		files.font = extra.font
	}
	return processImage('/api/v1/watermark', file, options, files)
}

// ---------- inspect ----------

export interface InspectResult {
	schema_version: string
	file: {
		path: string
		name: string
		ext: string
		size_bytes: number
		mime_type?: string
	}
	image?: {
		format: string
		width: number
		height: number
		aspect_ratio: string
		megapixels: number
		has_alpha: boolean
	}
	detail?: {
		extension_matches_format: boolean
	}
	hashes?: {
		sha256: string
		sha1: string
		md5: string
		crc32: string
	}
	error?: { code: string; message: string }
	warnings?: string[]
}

export async function inspectImage(file: File): Promise<InspectResult> {
	const form = new FormData()
	form.append('file', file)
	return requestJSON<InspectResult>('/api/v1/inspect', {
		method: 'POST',
		body: form,
	})
}
