import { Button, MenuItem, Slider, TextField, Typography } from '@mui/material'
import { useRef, useState } from 'react'
import type { WatermarkOptions } from '../api/client'
import { batchWatermark } from '../api/client'
import BatchResultPanel from '../components/BatchResultPanel'
import { useBatchTool } from '../hooks/useBatchTool'
import { BatchPanelShell } from './BatchPanels'
import type { FilesProp } from './types'

const POSITIONS = [
	'bottom-right',
	'bottom-left',
	'top-right',
	'top-left',
	'center',
]

export default function BatchWatermarkPanel({ files }: FilesProp) {
	const [type, setType] = useState<'text' | 'image'>('text')
	const [text, setText] = useState('lzwang')
	const [position, setPosition] = useState('bottom-right')
	const [opacity, setOpacity] = useState(0.5)
	const [color, setColor] = useState('')
	const [fontSize, setFontSize] = useState('')
	const [margin, setMargin] = useState(0.04)
	const [scale, setScale] = useState(0.2)
	const [watermarkFile, setWatermarkFile] = useState<File | null>(null)
	const [fontFile, setFontFile] = useState<File | null>(null)
	const wmInputRef = useRef<HTMLInputElement>(null)
	const fontInputRef = useRef<HTMLInputElement>(null)
	const { loading, error, result, run } = useBatchTool(batchWatermark)

	const process = () => {
		const options: WatermarkOptions = {
			type,
			text,
			mode: 'position',
			position,
			opacity,
			margin,
			scale,
			fontSize: Number(fontSize) || undefined,
			color: color.trim() !== '' ? color : undefined,
		}
		const extra: Record<string, File> = {}
		if (watermarkFile) {
			extra.watermark = watermarkFile
		}
		if (fontFile) {
			extra.font = fontFile
		}
		run(files, options, extra)
	}

	return (
		<BatchPanelShell
			title="批量水印 Batch Watermark"
			description="为所有图片添加同一水印，输出打包为 zip 下载"
			files={files}
			loading={loading}
			error={error}
			processLabel="开始批量水印"
			onProcess={process}
			result={result ? <BatchResultPanel result={result} /> : null}
		>
			<TextField
				select
				label="水印类型"
				value={type}
				onChange={(e) => setType(e.target.value as 'text' | 'image')}
			>
				<MenuItem value="text">文字水印</MenuItem>
				<MenuItem value="image">图片水印</MenuItem>
			</TextField>
			{type === 'text' ? (
				<>
					<TextField
						label="水印文字"
						value={text}
						onChange={(e) => setText(e.target.value)}
					/>
					<TextField
						label="字体大小（0=自动）"
						value={fontSize}
						onChange={(e) => setFontSize(e.target.value)}
					/>
					<TextField
						label="颜色（空=按背景亮度自动）"
						value={color}
						onChange={(e) => setColor(e.target.value)}
						placeholder="#FFFFFF"
					/>
				</>
			) : (
				<>
					<Typography variant="body2">
						缩放比例（相对底图短边）：{scale}
					</Typography>
					<Slider
						value={scale}
						min={0.05}
						max={1}
						step={0.05}
						onChange={(_, v) => setScale(v as number)}
					/>
					<Button
						variant="outlined"
						onClick={() => wmInputRef.current?.click()}
					>
						{watermarkFile ? `水印图片：${watermarkFile.name}` : '选择水印图片'}
					</Button>
					<input
						ref={wmInputRef}
						type="file"
						accept="image/*"
						hidden
						onChange={(e) => {
							setWatermarkFile(e.target.files?.[0] ?? null)
							e.target.value = ''
						}}
					/>
				</>
			)}
			<TextField
				select
				label="位置"
				value={position}
				onChange={(e) => setPosition(e.target.value)}
			>
				{POSITIONS.map((p) => (
					<MenuItem key={p} value={p}>
						{p}
					</MenuItem>
				))}
			</TextField>
			<Typography variant="body2">透明度：{opacity.toFixed(2)}</Typography>
			<Slider
				value={opacity}
				min={0.05}
				max={1}
				step={0.05}
				onChange={(_, v) => setOpacity(v as number)}
			/>
			<Typography variant="body2">边距比例：{margin.toFixed(2)}</Typography>
			<Slider
				value={margin}
				min={0}
				max={0.3}
				step={0.01}
				onChange={(_, v) => setMargin(v as number)}
			/>
			<Button variant="outlined" onClick={() => fontInputRef.current?.click()}>
				{fontFile
					? `字体文件：${fontFile.name}`
					: '上传字体文件（可选，默认系统字体）'}
			</Button>
			<input
				ref={fontInputRef}
				type="file"
				accept=".ttf,.otf,font/ttf,font/otf"
				hidden
				onChange={(e) => {
					setFontFile(e.target.files?.[0] ?? null)
					e.target.value = ''
				}}
			/>
		</BatchPanelShell>
	)
}
