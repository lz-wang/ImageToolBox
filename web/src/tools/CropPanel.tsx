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
import { cropImage } from '../api/client'
import { useImageTool } from '../hooks/useImageTool'
import type { FileProp } from './types'

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

export default function CropPanel({ file, onResult }: FileProp) {
	const [anchor, setAnchor] = useState('center')
	const [width, setWidth] = useState('40%')
	const [height, setHeight] = useState('40%')
	const { loading, error, run } = useImageTool(cropImage, onResult)

	const process = () => run(file, { anchor, width, height })

	return (
		<Stack spacing={2}>
			<Typography variant="h6">裁剪 Crop</Typography>
			<Typography variant="body2" color="text.secondary">
				按锚点 + 百分比保留目标区域；left/right 只填宽度，top/bottom 只填高度
			</Typography>
			<TextField
				select
				label="锚点"
				value={anchor}
				onChange={(e) => setAnchor(e.target.value)}
			>
				{ANCHORS.map((a) => (
					<MenuItem key={a} value={a}>
						{a}
					</MenuItem>
				))}
			</TextField>
			<Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
				<TextField
					label="宽度百分比"
					value={width}
					onChange={(e) => setWidth(e.target.value)}
					fullWidth
				/>
				<TextField
					label="高度百分比"
					value={height}
					onChange={(e) => setHeight(e.target.value)}
					fullWidth
				/>
			</Stack>
			{error ? <Alert severity="error">{error}</Alert> : null}
			<Button
				variant="contained"
				startIcon={<PlayArrowIcon />}
				loading={loading}
				onClick={process}
			>
				{loading ? '处理中…' : '开始裁剪'}
			</Button>
		</Stack>
	)
}
