import { Alert, Button, Slider, Stack, Typography } from '@mui/material'
import { useEffect, useState } from 'react'
import { type CompressOptions, compressImage } from '../api/client'
import { useImageTool } from '../hooks/useImageTool'
import type { FileProp } from './types'

export function compressedFilename(filename: string, quality: number): string {
	const extensionIndex = filename.lastIndexOf('.')
	const baseName =
		extensionIndex > 0 ? filename.slice(0, extensionIndex) : filename
	const extension = extensionIndex > 0 ? filename.slice(extensionIndex) : ''
	return `${baseName}--q${quality}${extension}`
}

async function compressWithFilename(file: File, options: CompressOptions) {
	const result = await compressImage(file, options)
	return {
		...result,
		filename: compressedFilename(file.name, options.quality),
	}
}

export default function CompressPanel({
	file,
	onResult,
	onLoadingChange,
}: FileProp) {
	const [quality, setQuality] = useState(80)
	const { loading, error, run } = useImageTool(compressWithFilename, onResult)

	useEffect(() => {
		onLoadingChange?.(loading)
		return () => onLoadingChange?.(false)
	}, [loading, onLoadingChange])

	return (
		<Stack spacing={1}>
			<Stack
				direction="row"
				spacing={2}
				sx={{ alignItems: 'center', width: '100%' }}
			>
				<Typography sx={{ whiteSpace: 'nowrap' }}>
					压缩质量: {quality}%
				</Typography>
				<Slider
					value={quality}
					min={1}
					max={100}
					step={1}
					onChange={(_, v) => setQuality(v as number)}
					sx={{ flex: 1, minWidth: 120 }}
				/>
				<Button
					variant="contained"
					loading={loading}
					disabled={loading}
					onClick={() => run(file, { quality })}
					sx={{ whiteSpace: 'nowrap' }}
				>
					{loading ? '应用中…' : '应用'}
				</Button>
			</Stack>
			{error ? <Alert severity="error">{error}</Alert> : null}
		</Stack>
	)
}
