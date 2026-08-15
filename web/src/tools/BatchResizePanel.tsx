import { MenuItem, Stack, TextField } from '@mui/material'
import { useState } from 'react'
import { batchResize } from '../api/client'
import BatchResultPanel from '../components/BatchResultPanel'
import { useBatchTool } from '../hooks/useBatchTool'
import { BatchPanelShell } from './BatchPanels'
import type { FilesProp } from './types'

const MODES = [
	{ value: 'fit', label: 'fit（等比缩放）' },
	{ value: 'fill', label: 'fill（裁剪填充）' },
	{ value: 'stretch', label: 'stretch（拉伸）' },
]
const FILTERS = ['lanczos', 'nearest', 'linear', 'catmullrom']

export default function BatchResizePanel({ files }: FilesProp) {
	const [width, setWidth] = useState('1200')
	const [height, setHeight] = useState('')
	const [percent, setPercent] = useState('')
	const [mode, setMode] = useState('fit')
	const [filter, setFilter] = useState('lanczos')
	const { loading, error, result, run } = useBatchTool(batchResize)

	return (
		<BatchPanelShell
			title="批量缩放 Batch Resize"
			description="复用 batch.Process 并发处理，输出打包为 zip 下载"
			files={files}
			loading={loading}
			error={error}
			processLabel="开始批量缩放"
			onProcess={() =>
				run(files, {
					width: Number(width) || 0,
					height: Number(height) || 0,
					percent,
					mode,
					anchor: 'center',
					filter,
				})
			}
			result={result ? <BatchResultPanel result={result} /> : null}
		>
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
				label="按比例缩放（如 50%）"
				value={percent}
				onChange={(e) => setPercent(e.target.value)}
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
		</BatchPanelShell>
	)
}
