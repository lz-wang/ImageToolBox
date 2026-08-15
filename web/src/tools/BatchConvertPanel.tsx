import {
	FormControlLabel,
	MenuItem,
	Slider,
	Switch,
	TextField,
	Typography,
} from '@mui/material'
import { useState } from 'react'
import { batchConvert } from '../api/client'
import BatchResultPanel from '../components/BatchResultPanel'
import { useBatchTool } from '../hooks/useBatchTool'
import { BatchPanelShell } from './BatchPanels'
import type { FilesProp } from './types'

const TARGETS = [
	{ value: 'webp', label: 'WebP' },
	{ value: 'png', label: 'PNG' },
	{ value: 'jpg', label: 'JPEG' },
]

export default function BatchConvertPanel({ files }: FilesProp) {
	const [to, setTo] = useState('webp')
	const [quality, setQuality] = useState(80)
	const [lossless, setLossless] = useState(false)
	const [background, setBackground] = useState('#FFFFFF')
	const { loading, error, result, run } = useBatchTool(batchConvert)

	return (
		<BatchPanelShell
			title="批量转换 Batch Convert"
			description="全部图片转换为统一目标格式，输出打包为 zip 下载"
			files={files}
			loading={loading}
			error={error}
			processLabel="开始批量转换"
			onProcess={() => run(files, { to, quality, lossless, background })}
			result={result ? <BatchResultPanel result={result} /> : null}
		>
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
		</BatchPanelShell>
	)
}
