import CloudUploadIcon from '@mui/icons-material/CloudUpload'
import DeleteIcon from '@mui/icons-material/Delete'
import { Box, Chip, IconButton, Stack, Typography } from '@mui/material'
import type { DragEvent } from 'react'
import { useRef, useState } from 'react'
import { formatBytes } from '../lib/format'
import { isImageFile } from '../lib/validate'

interface BatchDropzoneProps {
	files: File[]
	onChange: (files: File[]) => void
}

/** 多文件上传区：点击选择或拖拽放入，chip 列表可逐个移除 */
export default function BatchDropzone({ files, onChange }: BatchDropzoneProps) {
	const inputRef = useRef<HTMLInputElement>(null)
	const [dragOver, setDragOver] = useState(false)

	const addFiles = (incoming: FileList | File[] | null) => {
		const images = Array.from(incoming ?? []).filter(isImageFile)
		if (images.length === 0) {
			return
		}
		// 按名字去重追加
		const existing = new Set(files.map((f) => f.name))
		onChange([...files, ...images.filter((f) => !existing.has(f.name))])
	}

	const acceptDrop = (event: DragEvent<HTMLDivElement>) => {
		event.preventDefault()
		setDragOver(false)
		addFiles(event.dataTransfer.files)
	}

	return (
		<Stack spacing={1.5}>
			<Box
				onDragOver={(e) => {
					e.preventDefault()
					setDragOver(true)
				}}
				onDragLeave={() => setDragOver(false)}
				onDrop={acceptDrop}
				onClick={() => inputRef.current?.click()}
				sx={{
					border: '2px dashed',
					borderColor: dragOver ? 'primary.main' : 'divider',
					borderRadius: 2,
					py: 3,
					display: 'flex',
					flexDirection: 'column',
					alignItems: 'center',
					cursor: 'pointer',
					textAlign: 'center',
				}}
			>
				<input
					ref={inputRef}
					type="file"
					accept="image/*"
					multiple
					hidden
					onChange={(e) => {
						addFiles(e.target.files)
						e.target.value = ''
					}}
				/>
				<CloudUploadIcon color="action" sx={{ fontSize: 36 }} />
				<Typography color="text.secondary">点击选择或拖拽多张图片</Typography>
			</Box>
			{files.length > 0 ? (
				<Stack direction="row" spacing={1} useFlexGap sx={{ flexWrap: 'wrap' }}>
					{files.map((file, index) => (
						<Chip
							key={file.name}
							size="small"
							label={`${file.name} · ${formatBytes(file.size)}`}
							onDelete={() => onChange(files.filter((_, i) => i !== index))}
						/>
					))}
					<IconButton size="small" onClick={() => onChange([])} title="清空">
						<DeleteIcon fontSize="small" />
					</IconButton>
				</Stack>
			) : null}
		</Stack>
	)
}
