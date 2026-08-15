import DownloadIcon from '@mui/icons-material/Download'
import { Box, Button, Paper, Stack, Tooltip, Typography } from '@mui/material'
import { useEffect, useMemo, useState } from 'react'
import type { ProcessResult } from '../api/client'
import { formatBytes } from '../lib/format'
import ImageLightbox from './ImageLightbox'

interface ResultPanelProps {
	result: ProcessResult
}

interface ImageDimensions {
	width: number
	height: number
}

function imageFormat(result: ProcessResult): string {
	const mimeFormat = result.contentType.split('/')[1]
	if (mimeFormat) {
		return mimeFormat.toUpperCase()
	}
	return result.filename.split('.').pop()?.toUpperCase() ?? '图片'
}

function aspectRatio(width: number, height: number): string {
	let a = width
	let b = height
	while (b !== 0) {
		;[a, b] = [b, a % b]
	}
	return `${width / a}:${height / a}`
}

/** 处理结果卡片：与原图卡片使用相同的标题、预览和状态栏结构。 */
export default function ResultPanel({ result }: ResultPanelProps) {
	const [lightboxOpen, setLightboxOpen] = useState(false)
	const [dimensions, setDimensions] = useState<ImageDimensions | null>(null)
	const resultFile = useMemo(
		() =>
			new File([result.blob], result.filename, {
				type: result.contentType || result.blob.type,
			}),
		[result.blob, result.contentType, result.filename],
	)
	const metadata = dimensions
		? `${imageFormat(result)}，${dimensions.width}×${dimensions.height} (${aspectRatio(dimensions.width, dimensions.height)})，${formatBytes(result.outputSize || result.blob.size)}`
		: `${imageFormat(result)}，读取图片信息中…，${formatBytes(result.outputSize || result.blob.size)}`

	useEffect(() => {
		setDimensions(null)
		const image = new Image()
		image.onload = () => {
			setDimensions({ width: image.naturalWidth, height: image.naturalHeight })
		}
		image.onerror = () => setDimensions(null)
		image.src = result.url
		return () => {
			image.onload = null
			image.onerror = null
		}
	}, [result.url])

	return (
		<Paper
			elevation={1}
			sx={{
				width: 520,
				maxWidth: '100%',
				boxSizing: 'border-box',
				borderRadius: 2,
				p: 0,
				position: 'relative',
				display: 'flex',
				flexDirection: 'column',
				overflow: 'hidden',
			}}
		>
			<Typography
				variant="body2"
				noWrap
				sx={{
					width: '100%',
					px: 2,
					py: 1.5,
					fontWeight: 600,
					textAlign: 'center',
					borderBottom: 1,
					borderColor: 'divider',
					cursor: 'default',
				}}
			>
				{result.filename}
			</Typography>
			<Box
				sx={{
					width: '100%',
					aspectRatio: '1 / 1',
					boxSizing: 'border-box',
					p: 2,
					minWidth: 0,
					minHeight: 0,
					flex: '0 0 auto',
					overflow: 'hidden',
					borderBottom: 1,
					borderColor: 'divider',
					display: 'flex',
					alignItems: 'center',
					justifyContent: 'center',
				}}
			>
				<Tooltip title="点击查看">
					<Box
						component="img"
						src={result.url}
						alt={result.filename}
						role="button"
						tabIndex={0}
						onClick={() => setLightboxOpen(true)}
						onKeyDown={(event) => {
							if (event.key === 'Enter' || event.key === ' ') {
								event.preventDefault()
								setLightboxOpen(true)
							}
						}}
						sx={{
							maxWidth: '100%',
							maxHeight: '100%',
							minWidth: 0,
							minHeight: 0,
							objectFit: 'contain',
							borderRadius: 1,
							cursor: 'zoom-in',
						}}
					/>
				</Tooltip>
			</Box>
			<Stack
				direction="row"
				sx={{
					width: '100%',
					minHeight: 52,
					px: 2,
					alignItems: 'center',
					justifyContent: 'space-between',
				}}
			>
				<Typography
					variant="caption"
					color="text.secondary"
					noWrap
					sx={{
						minWidth: 0,
						flex: 1,
						mr: 1,
						textAlign: 'left',
						cursor: 'default',
					}}
				>
					{metadata}
				</Typography>
				<Button
					size="small"
					startIcon={<DownloadIcon fontSize="small" />}
					href={result.url}
					download={result.filename}
				>
					下载
				</Button>
			</Stack>
			<ImageLightbox
				file={resultFile}
				imageUrl={result.url}
				open={lightboxOpen}
				onClose={() => setLightboxOpen(false)}
			/>
		</Paper>
	)
}
