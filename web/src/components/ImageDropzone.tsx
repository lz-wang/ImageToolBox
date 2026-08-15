import CloudUploadIcon from '@mui/icons-material/CloudUpload'
import DeleteIcon from '@mui/icons-material/Delete'
import {
	Box,
	IconButton,
	Paper,
	Stack,
	Tooltip,
	Typography,
} from '@mui/material'
import type { DragEvent } from 'react'
import { useRef, useState } from 'react'
import { useObjectUrl } from '../hooks/useObjectUrl'
import { isImageFile } from '../lib/validate'

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
			onClick={() => inputRef.current?.click()}
			sx={{
				width: 520,
				height: 520,
				maxWidth: '100%',
				boxSizing: 'border-box',
				border: file ? 'none' : '2px dashed',
				borderColor: dragOver ? 'primary.main' : 'divider',
				borderRadius: 2,
				p: 2,
				bgcolor: file ? undefined : 'transparent',
				position: 'relative',
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
				<>
					<Box
						component="img"
						src={previewUrl}
						alt={file.name}
						sx={{
							maxWidth: '100%',
							maxHeight: '100%',
							objectFit: 'contain',
							borderRadius: 1,
						}}
					/>
					<Tooltip title="移除图片">
						<IconButton
							size="small"
							aria-label="移除图片"
							sx={{
								position: 'absolute',
								right: 16,
								bottom: 16,
								color: 'error.main',
								'&:hover': { bgcolor: 'transparent', color: 'error.dark' },
							}}
							onClick={(e) => {
								e.stopPropagation()
								onChange(null)
							}}
						>
							<DeleteIcon fontSize="small" />
						</IconButton>
					</Tooltip>
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
		</Paper>
	)
}
