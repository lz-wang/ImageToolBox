import { useEffect, useState } from 'react'

/** 为 File/Blob 创建 object URL，并在替换/卸载时自动回收 */
export function useObjectUrl(file: File | Blob | null): string {
	const [url, setUrl] = useState('')

	useEffect(() => {
		if (!file) {
			setUrl('')
			return
		}
		const created = URL.createObjectURL(file)
		setUrl(created)
		return () => URL.revokeObjectURL(created)
	}, [file])

	return url
}
