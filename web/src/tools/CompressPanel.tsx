import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import { Alert, Button, Slider, Stack, Typography } from '@mui/material'
import { useState } from 'react'
import { compressImage } from '../api/client'
import { useImageTool } from '../hooks/useImageTool'
import type { FileProp } from './types'

export default function CompressPanel({ file, onResult }: FileProp) {
	const [quality, setQuality] = useState(80)
	const { loading, error, run } = useImageTool(compressImage, onResult)

	return (
		<Stack spacing={2}>
			<Typography variant="h6">压缩 Compress</Typography>
			<Typography variant="body2" color="text.secondary">
				自动检测 PNG/JPEG 并使用 pngquant + oxipng / libjpeg-turbo 压缩
			</Typography>
			<Typography variant="body2">质量：{quality}</Typography>
			<Slider
				value={quality}
				min={10}
				max={100}
				step={5}
				onChange={(_, v) => setQuality(v as number)}
			/>
			{error ? <Alert severity="error">{error}</Alert> : null}
			<Button
				variant="contained"
				startIcon={<PlayArrowIcon />}
				loading={loading}
				disabled={loading}
				onClick={() => run(file, { quality })}
			>
				{loading ? '处理中…' : '开始压缩'}
			</Button>
		</Stack>
	)
}
