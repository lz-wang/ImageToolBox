import CloseIcon from '@mui/icons-material/Close'
import {
	Box,
	Dialog,
	IconButton,
	Stack,
	Tooltip,
	Typography,
} from '@mui/material'
import { alpha } from '@mui/material/styles'
import { useEffect, useState } from 'react'
import type { InspectResult } from '../api/client'
import { inspectImage } from '../api/client'
import { formatBytes } from '../lib/format'

interface ImageLightboxProps {
	file: File
	imageUrl: string
	open: boolean
	onClose: () => void
}

/** 原图与处理结果共用的全屏图片预览。 */
export default function ImageLightbox({
	file,
	imageUrl,
	open,
	onClose,
}: ImageLightboxProps) {
	const [inspectResult, setInspectResult] = useState<InspectResult | null>(null)
	const [inspectLoading, setInspectLoading] = useState(false)
	const [inspectError, setInspectError] = useState('')

	useEffect(() => {
		if (!open) return

		let cancelled = false
		setInspectLoading(true)
		setInspectError('')
		setInspectResult(null)
		void inspectImage(file)
			.then((result) => {
				if (!cancelled) setInspectResult(result)
			})
			.catch((error) => {
				if (!cancelled) {
					setInspectError(
						error instanceof Error ? error.message : String(error),
					)
				}
			})
			.finally(() => {
				if (!cancelled) setInspectLoading(false)
			})

		return () => {
			cancelled = true
		}
	}, [file, open])

	return (
		<Dialog
			fullScreen
			open={open}
			onClose={onClose}
			aria-label="图片预览"
			slotProps={{
				paper: {
					sx: {
						bgcolor: 'rgba(0, 0, 0, 0.92)',
						m: 0,
						position: 'relative',
						display: 'flex',
						alignItems: 'center',
						justifyContent: 'center',
					},
				},
			}}
		>
			<Box
				component="img"
				src={imageUrl}
				alt={file.name}
				sx={{
					maxWidth: 'calc(100vw - 64px)',
					maxHeight: 'calc(100vh - 64px)',
					m: 'auto',
					objectFit: 'contain',
				}}
			/>
			<Tooltip title="关闭预览">
				<IconButton
					aria-label="关闭图片预览"
					onClick={onClose}
					sx={{
						position: 'absolute',
						top: 12,
						right: 12,
						color: 'text.primary',
					}}
				>
					<CloseIcon />
				</IconButton>
			</Tooltip>
			<Box
				sx={(theme) => ({
					position: 'absolute',
					top: 64,
					right: 24,
					width: 280,
					maxWidth: 'calc(100vw - 48px)',
					maxHeight: 'calc(100vh - 88px)',
					overflowY: 'auto',
					p: 2,
					borderRadius: 2,
					bgcolor: alpha(theme.palette.background.paper, 0.86),
					backdropFilter: 'blur(12px)',
					boxShadow: 4,
				})}
			>
				<Typography variant="subtitle2" sx={{ mb: 1.25 }}>
					图片信息
				</Typography>
				{inspectLoading ? (
					<Typography variant="body2" color="text.secondary">
						正在读取图片信息…
					</Typography>
				) : null}
				{inspectError ? (
					<Typography variant="body2" color="error">
						{inspectError}
					</Typography>
				) : null}
				{inspectResult ? <InspectDetails result={inspectResult} /> : null}
			</Box>
		</Dialog>
	)
}

function InspectDetails({ result }: { result: InspectResult }) {
	return (
		<Stack spacing={0.75}>
			<LightboxInfo label="文件名" value={result.file.name} />
			<LightboxInfo label="大小" value={formatBytes(result.file.size_bytes)} />
			<LightboxInfo label="MIME" value={result.file.mime_type ?? '-'} />
			{result.image ? (
				<>
					<LightboxInfo
						label="格式"
						value={result.image.format.toUpperCase()}
					/>
					<LightboxInfo
						label="尺寸"
						value={`${result.image.width}×${result.image.height}`}
					/>
					<LightboxInfo label="宽高比" value={result.image.aspect_ratio} />
					<LightboxInfo
						label="像素"
						value={`${result.image.megapixels.toFixed(2)} MP`}
					/>
					<LightboxInfo
						label="透明通道"
						value={result.image.has_alpha ? '有' : '无'}
					/>
				</>
			) : null}
			{result.detail ? (
				<LightboxInfo
					label="扩展名匹配"
					value={result.detail.extension_matches_format ? '匹配' : '不匹配'}
				/>
			) : null}
			{result.hashes ? (
				<>
					<LightboxInfo label="SHA256" value={result.hashes.sha256} mono />
					<LightboxInfo label="MD5" value={result.hashes.md5} mono />
				</>
			) : null}
		</Stack>
	)
}

function LightboxInfo({
	label,
	value,
	mono = false,
}: {
	label: string
	value: string
	mono?: boolean
}) {
	return (
		<Typography
			variant="body2"
			sx={{
				overflowWrap: 'anywhere',
				fontFamily: mono ? 'monospace' : undefined,
			}}
		>
			<Box component="span" sx={{ color: 'text.secondary' }}>
				{label}:
			</Box>
			{value}
		</Typography>
	)
}
