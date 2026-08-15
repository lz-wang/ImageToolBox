import CloudUploadIcon from '@mui/icons-material/CloudUpload'
import RestartAltIcon from '@mui/icons-material/RestartAlt'
import {
	Box,
	Button,
	Dialog,
	Paper,
	Stack,
	Tooltip,
	Typography,
} from '@mui/material'
import type { DragEvent } from 'react'
import { useEffect, useRef, useState } from 'react'
import { useObjectUrl } from '../hooks/useObjectUrl'
import { formatBytes } from '../lib/format'
import { isImageFile } from '../lib/validate'

interface ImageDropzoneProps {
	file: File | null
	onChange: (file: File | null) => void
}

interface ImageDimensions {
	width: number
	height: number
}

function imageFormat(file: File): string {
	const mimeFormat = file.type.split('/')[1]
	if (mimeFormat) {
		return mimeFormat.toUpperCase()
	}
	return file.name.split('.').pop()?.toUpperCase() ?? '图片'
}

function aspectRatio(width: number, height: number): string {
	let a = width
	let b = height
	while (b !== 0) {
		;[a, b] = [b, a % b]
	}
	return `${width / a}:${height / a}`
}

/** 图片上传区：点击选择或拖拽放入，带预览。原生 File API，不引入 dropzone 库 */
export default function ImageDropzone({ file, onChange }: ImageDropzoneProps) {
	const inputRef = useRef<HTMLInputElement>(null)
	const [dragOver, setDragOver] = useState(false)
	const [lightboxOpen, setLightboxOpen] = useState(false)
	const [dimensions, setDimensions] = useState<ImageDimensions | null>(null)
	const previewUrl = useObjectUrl(file)
	const metadata =
		file && dimensions
			? `${imageFormat(file)}，${dimensions.width}×${dimensions.height} (${aspectRatio(dimensions.width, dimensions.height)})，${formatBytes(file.size)}`
			: file
				? `${imageFormat(file)}，读取图片信息中…，${formatBytes(file.size)}`
				: ''

	useEffect(() => {
		if (!previewUrl) {
			setDimensions(null)
			return
		}

		const image = new Image()
		image.onload = () => {
			setDimensions({ width: image.naturalWidth, height: image.naturalHeight })
		}
		image.onerror = () => setDimensions(null)
		image.src = previewUrl
		return () => {
			image.onload = null
			image.onerror = null
		}
	}, [previewUrl])

	const acceptDrop = (event: DragEvent<HTMLDivElement>) => {
		event.preventDefault()
		setDragOver(false)
		const dropped = event.dataTransfer.files?.[0]
		if (dropped && isImageFile(dropped)) {
			onChange(dropped)
		}
	}

	return (
		<Paper
			elevation={file ? 1 : 0}
			onDragOver={(e) => {
				e.preventDefault()
				setDragOver(true)
			}}
			onDragLeave={() => setDragOver(false)}
			onDrop={acceptDrop}
			onClick={() => {
				if (!file) inputRef.current?.click()
			}}
			sx={{
				width: 520,
				height: file ? 'auto' : 520,
				maxWidth: '100%',
				boxSizing: 'border-box',
				border: file ? 'none' : '2px dashed',
				borderColor: dragOver ? 'primary.main' : 'divider',
				borderRadius: 2,
				p: file ? 0 : 2,
				bgcolor: file ? undefined : 'transparent',
				position: 'relative',
				display: 'flex',
				flexDirection: 'column',
				alignItems: 'center',
				justifyContent: 'center',
				cursor: file ? 'default' : 'pointer',
				textAlign: 'center',
				overflow: 'hidden',
				transition: 'border-color .2s',
			}}
		>
			<input
				ref={inputRef}
				type="file"
				accept="image/*"
				hidden
				onChange={(e) => {
					const selected = e.target.files?.[0] ?? null
					if (selected) {
						onChange(selected)
					}
					e.target.value = ''
				}}
			/>
			{file ? (
				<>
					<Typography
						variant="body2"
						noWrap
						sx={{
							width: '100%',
							px: 2,
							py: 1.5,
							fontWeight: 600,
							borderBottom: 1,
							borderColor: 'divider',
							cursor: 'default',
						}}
						onClick={(event) => event.stopPropagation()}
					>
						{file.name}
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
								src={previewUrl}
								alt={file.name}
								role="button"
								tabIndex={0}
								onClick={(event) => {
									event.stopPropagation()
									setLightboxOpen(true)
								}}
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
						onClick={(event) => event.stopPropagation()}
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
							color="error"
							startIcon={<RestartAltIcon fontSize="small" />}
							onClick={(e) => {
								e.stopPropagation()
								setLightboxOpen(false)
								onChange(null)
							}}
						>
							重置
						</Button>
					</Stack>
				</>
			) : (
				<Stack spacing={1} sx={{ alignItems: 'center', py: 4 }}>
					<CloudUploadIcon color="action" sx={{ fontSize: 48 }} />
					<Typography color="text.secondary">
						点击选择或拖拽图片到此处
					</Typography>
					<Typography variant="caption" color="text.secondary">
						支持 JPEG / PNG / WEBP
					</Typography>
				</Stack>
			)}
			<Dialog
				fullScreen
				open={lightboxOpen}
				onClose={() => setLightboxOpen(false)}
				onClick={(event) => event.stopPropagation()}
				aria-label="图片预览"
				slotProps={{
					paper: {
						sx: {
							bgcolor: 'rgba(0, 0, 0, 0.92)',
							m: 0,
						},
					},
				}}
			>
				<Box
					component="img"
					src={previewUrl}
					alt={file?.name ?? '图片预览'}
					sx={{
						maxWidth: 'calc(100vw - 64px)',
						maxHeight: 'calc(100vh - 64px)',
						m: 'auto',
						objectFit: 'contain',
					}}
				/>
			</Dialog>
		</Paper>
	)
}
