import { Box, Chip, Stack, Typography } from '@mui/material'
import { useEffect, useState } from 'react'
import { formatBytes } from '../lib/format'

interface Dims {
	w: number
	h: number
}

export function useImageSize(url: string): Dims | null {
	const [dims, setDims] = useState<Dims | null>(null)

	useEffect(() => {
		if (!url) {
			setDims(null)
			return
		}
		const img = new Image()
		img.onload = () => setDims({ w: img.naturalWidth, h: img.naturalHeight })
		img.src = url
		return () => {
			img.onload = null
		}
	}, [url])

	return dims
}

interface CompareItemProps {
	label: string
	url: string
	name: string
	size: number
	dims: Dims | null
}

function CompareItem({ label, url, name, size, dims }: CompareItemProps) {
	return (
		<Box sx={{ flex: 1, minWidth: 0 }}>
			<Stack direction="row" spacing={1} sx={{ alignItems: 'center', mb: 0.5 }}>
				<Typography variant="subtitle2">{label}</Typography>
				{dims ? <Chip size="small" label={`${dims.w}×${dims.h}`} /> : null}
				<Chip size="small" label={formatBytes(size)} />
			</Stack>
			<Box
				component="img"
				src={url}
				alt={name}
				sx={{
					maxWidth: '100%',
					maxHeight: 360,
					borderRadius: 1,
					border: '1px solid',
					borderColor: 'divider',
					display: 'block',
				}}
			/>
		</Box>
	)
}

interface BeforeAfterProps {
	sourceUrl: string
	sourceName: string
	inputSize: number
	outputUrl: string
	outputName: string
	outputSize: number
}

/** Before / After 对比视图：左右并排展示尺寸与体积变化 */
export default function BeforeAfter({
	sourceUrl,
	sourceName,
	inputSize,
	outputUrl,
	outputName,
	outputSize,
}: BeforeAfterProps) {
	const before = useImageSize(sourceUrl)
	const after = useImageSize(outputUrl)

	const changed =
		inputSize > 0 ? Math.round((1 - outputSize / inputSize) * 100) : 0

	return (
		<Stack spacing={1.5}>
			<Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
				<CompareItem
					label="原图"
					url={sourceUrl}
					name={sourceName}
					size={inputSize}
					dims={before}
				/>
				<CompareItem
					label="结果"
					url={outputUrl}
					name={outputName}
					size={outputSize}
					dims={after}
				/>
			</Stack>
			{before && after ? (
				<Typography variant="body2" color="text.secondary">
					{before.w}×{before.h} → {after.w}×{after.h} ｜{' '}
					{formatBytes(inputSize)} → {formatBytes(outputSize)}
				</Typography>
			) : null}
			{changed > 0 ? (
				<Chip
					size="small"
					color="success"
					label={`体积减少 ${changed}%`}
					sx={{ alignSelf: 'flex-start' }}
				/>
			) : null}
			{changed < 0 ? (
				<Chip
					size="small"
					color="warning"
					label={`体积增加 ${-changed}%`}
					sx={{ alignSelf: 'flex-start' }}
				/>
			) : null}
		</Stack>
	)
}
