import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import {
	Alert,
	Button,
	FormControlLabel,
	MenuItem,
	Slider,
	Stack,
	Switch,
	TextField,
	Typography,
} from '@mui/material'
import { useState } from 'react'
import { convertImage } from '../api/client'
import ResultPanel from '../components/ResultPanel'
import { useImageTool } from '../hooks/useImageTool'
import type { FileProp } from './types'

const TARGETS = [
	{ value: 'webp', label: 'WebP' },
	{ value: 'png', label: 'PNG' },
	{ value: 'jpg', label: 'JPEG' },
]

export default function ConvertPanel({ file }: FileProp) {
	const [to, setTo] = useState('webp')
	const [quality, setQuality] = useState(80)
	const [lossless, setLossless] = useState(false)
	const [background, setBackground] = useState('#FFFFFF')
	const { loading, error, result, run } = useImageTool(convertImage)

	const process = () => run(file, { to, quality, lossless, background })

	return (
		<Stack spacing={2}>
			<Typography variant="h6">格式转换 Convert</Typography>
			<TextField
				select
				label="目标格式"
				value={to}
				onChange={(e) => setTo(e.target.value)}
			>
				{TARGETS.map((t) => (
					<MenuItem key={t.value} value={t.value}>
						{t.label}
					</MenuItem>
				))}
			</TextField>
			<Typography variant="body2">质量：{quality}</Typography>
			<Slider
				value={quality}
				min={10}
				max={100}
				step={5}
				onChange={(_, v) => setQuality(v as number)}
			/>
			<FormControlLabel
				control={
					<Switch
						checked={lossless}
						onChange={(e) => setLossless(e.target.checked)}
					/>
				}
				label="无损编码（WebP）"
			/>
			<TextField
				label="透明图转不透明格式时的背景色"
				value={background}
				onChange={(e) => setBackground(e.target.value)}
			/>
			{error ? <Alert severity="error">{error}</Alert> : null}
			<Button
				variant="contained"
				startIcon={<PlayArrowIcon />}
				loading={loading}
				onClick={process}
			>
				{loading ? '处理中…' : '开始转换'}
			</Button>
			{result ? <ResultPanel result={result} /> : null}
		</Stack>
	)
}
