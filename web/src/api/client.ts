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

function filenameFromDisposition(
	disposition: string,
	fallback: string,
): string {
	const encodedMatch = /filename\*=UTF-8''([^;]+)/i.exec(disposition)
	if (encodedMatch) {
		try {
			return decodeURIComponent(encodedMatch[1])
		} catch {
			// Fall back to the legacy filename parameter when decoding fails.
		}
	}

	const filenameMatch = /filename="?([^";]+)"?/i.exec(disposition)
	return filenameMatch?.[1] ?? fallback
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
	return {
		blob,
		url: URL.createObjectURL(blob),
		contentType: response.headers.get('Content-Type') ?? blob.type,
		filename: filenameFromDisposition(disposition, 'result'),
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

// ---------- 批量处理 ----------

export interface BatchProcessResult {
	blob: Blob
	url: string
	filename: string
	success: number
	skipped: number
	failed: number
}

async function processBatch(
	path: string,
	files: File[],
	options: unknown,
	extra?: Record<string, File>,
): Promise<BatchProcessResult> {
	const form = new FormData()
	for (const file of files) {
		form.append('files', file)
	}
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
	return {
		blob,
		url: URL.createObjectURL(blob),
		filename: filenameFromDisposition(disposition, 'itb-batch-result.zip'),
		success: Number(response.headers.get('X-ITB-Success') ?? 0),
		skipped: Number(response.headers.get('X-ITB-Skipped') ?? 0),
		failed: Number(response.headers.get('X-ITB-Failed') ?? 0),
	}
}

export function batchResize(
	files: File[],
	options: ResizeOptions,
): Promise<BatchProcessResult> {
	return processBatch('/api/v1/batch/resize', files, options)
}

export function batchConvert(
	files: File[],
	options: ConvertOptions,
): Promise<BatchProcessResult> {
	return processBatch('/api/v1/batch/convert', files, options)
}

export function batchWatermark(
	files: File[],
	options: WatermarkOptions,
	extra: { watermark?: File; font?: File } = {},
): Promise<BatchProcessResult> {
	const extraFiles: Record<string, File> = {}
	if (extra.watermark) {
		extraFiles.watermark = extra.watermark
	}
	if (extra.font) {
		extraFiles.font = extra.font
	}
	return processBatch('/api/v1/batch/watermark', files, options, extraFiles)
}

// ---------- 存储（S3 / Lsky，凭证仅存于服务端环境变量） ----------

export interface S3Status {
	configured: boolean
	endpoint: string
	region: string
	bucket: string
}

export function fetchS3Status(): Promise<S3Status> {
	return requestJSON<S3Status>('/api/v1/s3/status')
}

export interface S3Object {
	key: string
	size: number
	last_modified: string
	etag: string
	storage_class: string
}

export async function fetchS3Objects(prefix = ''): Promise<S3Object[]> {
	const query = prefix ? `?prefix=${encodeURIComponent(prefix)}` : ''
	const result = await requestJSON<{ objects: S3Object[] | null }>(
		`/api/v1/s3/objects${query}`,
	)
	return result.objects ?? []
}

export async function uploadS3Object(
	file: File,
	options: {
		key?: string
		prefix?: string
		skipExisting?: boolean
		skipUnchanged?: boolean
	},
): Promise<{ key: string; skipped?: boolean; reason?: string }> {
	const form = new FormData()
	form.append('file', file)
	form.append('options', JSON.stringify(options))
	return requestJSON<{ key: string; skipped?: boolean; reason?: string }>(
		'/api/v1/s3/objects',
		{
			method: 'POST',
			body: form,
		},
	)
}

export function s3DownloadUrl(key: string): string {
	return `/api/v1/s3/objects/download?key=${encodeURIComponent(key)}`
}

/** 单对象完整元数据（HeadObject），仅在"查看详情"时请求；列表展示一律用 S3Object */
export interface S3ObjectStat {
	key: string
	size: number
	last_modified: string
	etag: string
	content_type?: string
	cache_control?: string
	content_disposition?: string
	content_encoding?: string
	storage_class?: string
	version_id?: string
	metadata?: Record<string, string>
}

export function fetchS3ObjectStat(key: string): Promise<S3ObjectStat> {
	return requestJSON<S3ObjectStat>(
		`/api/v1/s3/objects/info?key=${encodeURIComponent(key)}`,
	)
}

export async function deleteS3Object(key: string): Promise<void> {
	const response = await fetch(
		`/api/v1/s3/objects?key=${encodeURIComponent(key)}`,
		{
			method: 'DELETE',
		},
	)
	if (!response.ok) {
		throw new ApiError(await parseErrorMessage(response), response.status)
	}
}

export interface LskyUploadResult {
	name?: string
	url: string
	markdown: string
}

export async function uploadLskyImage(
	file: File,
	strategyId = 0,
): Promise<LskyUploadResult> {
	const form = new FormData()
	form.append('file', file)
	form.append('options', JSON.stringify({ strategyId }))
	return requestJSON<LskyUploadResult>('/api/v1/lsky/images', {
		method: 'POST',
		body: form,
	})
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
