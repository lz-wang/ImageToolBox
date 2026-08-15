import DownloadIcon from '@mui/icons-material/Download'
import { Box, Button, Chip, Stack } from '@mui/material'
import { useEffect, useState } from 'react'
import type { ProcessResult } from '../api/client'
import { formatBytes } from '../lib/format'

/** 处理结果展示：预览 + 尺寸 + 体积对比 + 下载 */
export default function ResultPanel({ result }: { result: ProcessResult }) {
	const [dims, setDims] = useState<{ w: number; h: number } | null>(null)

	useEffect(() => {
		const img = new Image()
		img.onload = () => setDims({ w: img.naturalWidth, h: img.naturalHeight })
		img.src = result.url
	}, [result])

	return (
		<Stack spacing={1.5}>
			<Box
				component="img"
				src={result.url}
				alt={result.filename}
				sx={{
					maxWidth: '100%',
					maxHeight: 420,
					alignSelf: 'flex-start',
					borderRadius: 1,
					border: '1px solid',
					borderColor: 'divider',
				}}
			/>
			<Stack
				direction="row"
				spacing={1}
				useFlexGap
				sx={{ flexWrap: 'wrap', alignItems: 'center' }}
			>
				<Chip label={result.filename} size="small" />
				{dims ? <Chip size="small" label={`${dims.w}×${dims.h}`} /> : null}
				<Chip
					size="small"
					label={`${formatBytes(result.inputSize)} → ${formatBytes(result.outputSize)}`}
				/>
				<Button
					variant="contained"
					size="small"
					startIcon={<DownloadIcon />}
					href={result.url}
					download={result.filename}
				>
					下载
				</Button>
			</Stack>
		</Stack>
	)
}
