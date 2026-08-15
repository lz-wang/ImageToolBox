import CloudUploadIcon from '@mui/icons-material/CloudUpload'
import { Box, Button, Stack, Typography } from '@mui/material'
import type { DragEvent } from 'react'
import { useRef, useState } from 'react'
import { useObjectUrl } from '../hooks/useObjectUrl'
import { formatBytes } from '../lib/format'

interface ImageDropzoneProps {
	file: File | null
	onChange: (file: File | null) => void
}

/** 图片上传区：点击选择或拖拽放入，带预览。原生 File API，不引入 dropzone 库 */
export default function ImageDropzone({ file, onChange }: ImageDropzoneProps) {
	const inputRef = useRef<HTMLInputElement>(null)
	const [dragOver, setDragOver] = useState(false)
	const previewUrl = useObjectUrl(file)

	const acceptDrop = (event: DragEvent<HTMLDivElement>) => {
		event.preventDefault()
		setDragOver(false)
		const dropped = event.dataTransfer.files?.[0]
		if (dropped?.type.startsWith('image/')) {
			onChange(dropped)
		}
	}

	return (
		<Box
			onDragOver={(e) => {
				e.preventDefault()
				setDragOver(true)
			}}
			onDragLeave={() => setDragOver(false)}
			onDrop={acceptDrop}
			onClick={() => inputRef.current?.click()}
			sx={{
				minHeight: 260,
				border: '2px dashed',
				borderColor: dragOver ? 'primary.main' : 'divider',
				borderRadius: 2,
				p: 2,
				display: 'flex',
				flexDirection: 'column',
				alignItems: 'center',
				justifyContent: 'center',
				cursor: 'pointer',
				textAlign: 'center',
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
				<Stack spacing={1} sx={{ alignItems: 'center' }}>
					<Box
						component="img"
						src={previewUrl}
						alt={file.name}
						sx={{ maxWidth: '100%', maxHeight: 320, borderRadius: 1 }}
					/>
					<Typography variant="body2" noWrap sx={{ maxWidth: '100%' }}>
						{file.name} · {formatBytes(file.size)}
					</Typography>
					<Button
						size="small"
						variant="outlined"
						onClick={(e) => {
							e.stopPropagation()
							onChange(null)
						}}
					>
						移除图片
					</Button>
				</Stack>
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
		</Box>
	)
}
