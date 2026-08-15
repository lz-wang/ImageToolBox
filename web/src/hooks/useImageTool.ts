import { useCallback, useEffect, useRef, useState } from 'react'
import type { ProcessResult } from '../api/client'

export interface ImageToolState<O> {
	loading: boolean
	error: string
	result: ProcessResult | null
	run: (file: File, options: O, extra?: Record<string, File>) => Promise<void>
	reset: () => void
}

/** 单图工具的公共状态：loading / error / result，并负责 object URL 的回收 */
export function useImageTool<O>(
	processor: (
		file: File,
		options: O,
		extra?: Record<string, File>,
	) => Promise<ProcessResult>,
): ImageToolState<O> {
	const [loading, setLoading] = useState(false)
	const [error, setError] = useState('')
	const [result, setResult] = useState<ProcessResult | null>(null)
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
		async (file: File, options: O, extra?: Record<string, File>) => {
			setLoading(true)
			setError('')
			if (urlRef.current) {
				URL.revokeObjectURL(urlRef.current)
				urlRef.current = null
			}
			setResult(null)
			try {
				const res = await processor(file, options, extra)
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

	const reset = useCallback(() => {
		if (urlRef.current) {
			URL.revokeObjectURL(urlRef.current)
			urlRef.current = null
		}
		setResult(null)
		setError('')
	}, [])

	return { loading, error, result, run, reset }
}
