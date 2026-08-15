import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import {
	Alert,
	Button,
	MenuItem,
	Stack,
	TextField,
	Typography,
} from '@mui/material'
import { useState } from 'react'
import { resizeImage } from '../api/client'
import { useImageTool } from '../hooks/useImageTool'
import type { FileProp } from './types'

const MODES = [
	{ value: 'fit', label: 'fit（等比缩放，不超出目标框）' },
	{ value: 'fill', label: 'fill（裁剪填充到目标尺寸）' },
	{ value: 'stretch', label: 'stretch（拉伸变形）' },
]

const ANCHORS = [
	'center',
	'left',
	'right',
	'top',
	'bottom',
	'top-left',
	'top-right',
	'bottom-left',
	'bottom-right',
]
const FILTERS = ['lanczos', 'nearest', 'linear', 'catmullrom']

export default function ResizePanel({ file, onResult }: FileProp) {
	const [width, setWidth] = useState('1200')
	const [height, setHeight] = useState('')
	const [percent, setPercent] = useState('')
	const [mode, setMode] = useState('fit')
	const [anchor, setAnchor] = useState('center')
	const [filter, setFilter] = useState('lanczos')
	const { loading, error, run } = useImageTool(resizeImage, onResult)

	const process = () => {
		run(file, {
			width: Number(width) || 0,
			height: Number(height) || 0,
			percent,
			mode,
			anchor,
			filter,
		})
	}

	return (
		<Stack spacing={2}>
			<Typography variant="h6">调整尺寸 Resize</Typography>
			<Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
				<TextField
					label="宽度 (px)"
					value={width}
					onChange={(e) => setWidth(e.target.value)}
					fullWidth
				/>
				<TextField
					label="高度 (px)"
					value={height}
					onChange={(e) => setHeight(e.target.value)}
					fullWidth
				/>
			</Stack>
			<TextField
				label="按比例缩放（如 50%，与宽高互斥）"
				value={percent}
				onChange={(e) => setPercent(e.target.value)}
				placeholder="50%"
			/>
			<TextField
				select
				label="模式"
				value={mode}
				onChange={(e) => setMode(e.target.value)}
			>
				{MODES.map((m) => (
					<MenuItem key={m.value} value={m.value}>
						{m.label}
					</MenuItem>
				))}
			</TextField>
			<TextField
				select
				label="锚点（fill 模式）"
				value={anchor}
				onChange={(e) => setAnchor(e.target.value)}
			>
				{ANCHORS.map((a) => (
					<MenuItem key={a} value={a}>
						{a}
					</MenuItem>
				))}
			</TextField>
			<TextField
				select
				label="采样器"
				value={filter}
				onChange={(e) => setFilter(e.target.value)}
			>
				{FILTERS.map((f) => (
					<MenuItem key={f} value={f}>
						{f}
					</MenuItem>
				))}
			</TextField>
			{error ? <Alert severity="error">{error}</Alert> : null}
			<Button
				variant="contained"
				startIcon={<PlayArrowIcon />}
				loading={loading}
				onClick={process}
			>
				{loading ? '处理中…' : '开始缩放'}
			</Button>
		</Stack>
	)
}
