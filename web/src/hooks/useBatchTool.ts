import { useCallback, useEffect, useRef, useState } from 'react'
import type { BatchProcessResult } from '../api/client'

export interface BatchToolState {
	loading: boolean
	error: string
	result: BatchProcessResult | null
	run: (
		files: File[],
		options: unknown,
		extra?: Record<string, File>,
	) => Promise<void>
}

/** 批量工具的公共状态：loading / error / result（zip），负责 object URL 回收 */
export function useBatchTool(
	processor: (
		files: File[],
		options: never,
		extra?: Record<string, File>,
	) => Promise<BatchProcessResult>,
): BatchToolState {
	const [loading, setLoading] = useState(false)
	const [error, setError] = useState('')
	const [result, setResult] = useState<BatchProcessResult | null>(null)
	const urlRef = useRef<string | null>(null)

	useEffect(
		() => () => {
			if (urlRef.current) {
				URL.revokeObjectURL(urlRef.current)
			}
		},
		[],
	)

	const run = useCallback(
		async (files: File[], options: unknown, extra?: Record<string, File>) => {
			setLoading(true)
			setError('')
			if (urlRef.current) {
				URL.revokeObjectURL(urlRef.current)
				urlRef.current = null
			}
			setResult(null)
			try {
				const res = await processor(files, options as never, extra)
				urlRef.current = res.url
				setResult(res)
			} catch (err) {
				setError(err instanceof Error ? err.message : String(err))
			} finally {
				setLoading(false)
			}
		},
		[processor],
	)

	return { loading, error, result, run }
}
