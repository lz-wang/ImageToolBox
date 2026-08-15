import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import {
	Alert,
	Button,
	MenuItem,
	Slider,
	Stack,
	TextField,
	ToggleButton,
	ToggleButtonGroup,
	Typography,
} from '@mui/material'
import { useRef, useState } from 'react'
import type { WatermarkOptions } from '../api/client'
import { watermarkImage } from '../api/client'
import ResultPanel from '../components/ResultPanel'
import { useImageTool } from '../hooks/useImageTool'
import { useObjectUrl } from '../hooks/useObjectUrl'
import type { FileProp } from './types'

const POSITIONS = [
	'bottom-right',
	'bottom-left',
	'top-right',
	'top-left',
	'center',
]

export default function WatermarkPanel({ file }: FileProp) {
	const [type, setType] = useState<'text' | 'image'>('text')
	const [text, setText] = useState('lzwang')
	const [mode, setMode] = useState<'position' | 'repeat'>('position')
	const [position, setPosition] = useState('bottom-right')
	const [opacity, setOpacity] = useState(0.5)
	const [color, setColor] = useState('')
	const [fontSize, setFontSize] = useState('')
	const [angle, setAngle] = useState(30)
	const [space, setSpace] = useState('')
	const [margin, setMargin] = useState(0.04)
	const [scale, setScale] = useState(0.2)
	const [watermarkFile, setWatermarkFile] = useState<File | null>(null)
	const [fontFile, setFontFile] = useState<File | null>(null)
	const wmInputRef = useRef<HTMLInputElement>(null)
	const fontInputRef = useRef<HTMLInputElement>(null)
	const sourceUrl = useObjectUrl(file)
	const { loading, error, result, run } = useImageTool(watermarkImage)

	const process = () => {
		const options: WatermarkOptions = {
			type,
			text,
			mode: type === 'image' ? 'position' : mode,
			position,
			opacity,
			margin,
			scale,
			fontSize: Number(fontSize) || undefined,
			space: Number(space) || undefined,
			angle,
			color: color.trim() !== '' ? color : undefined,
		}
		const extra: Record<string, File> = {}
		if (watermarkFile) {
			extra.watermark = watermarkFile
		}
		if (fontFile) {
			extra.font = fontFile
		}
		run(file, options, extra)
	}

	return (
		<Stack spacing={2}>
			<Typography variant="h6">水印 Watermark</Typography>
			<ToggleButtonGroup
				exclusive
				value={type}
				onChange={(_, v) => {
					if (v) setType(v)
				}}
			>
				<ToggleButton value="text">文字水印</ToggleButton>
				<ToggleButton value="image">图片水印</ToggleButton>
			</ToggleButtonGroup>

			{type === 'text' ? (
				<>
					<TextField
						label="水印文字"
						value={text}
						onChange={(e) => setText(e.target.value)}
					/>
					<TextField
						select
						label="模式"
						value={mode}
						onChange={(e) => setMode(e.target.value as 'position' | 'repeat')}
					>
						<MenuItem value="position">position（单点位置）</MenuItem>
						<MenuItem value="repeat">repeat（平铺）</MenuItem>
					</TextField>
					{mode === 'position' ? (
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
					) : null}
					<Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
						<TextField
							label="字体大小（0=自动）"
							value={fontSize}
							onChange={(e) => setFontSize(e.target.value)}
							fullWidth
						/>
						<TextField
							label="平铺间距（0=自动）"
							value={space}
							onChange={(e) => setSpace(e.target.value)}
							fullWidth
						/>
					</Stack>
					{mode === 'repeat' ? (
						<>
							<Typography variant="body2">旋转角度：{angle}°</Typography>
							<Slider
								value={angle}
								min={0}
								max={90}
								step={5}
								onChange={(_, v) => setAngle(v as number)}
							/>
						</>
					) : null}
					<TextField
						label="颜色（空=按背景亮度自动）"
						value={color}
						onChange={(e) => setColor(e.target.value)}
						placeholder="#FFFFFF"
					/>
				</>
			) : (
				<>
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

			{error ? <Alert severity="error">{error}</Alert> : null}
			<Button
				variant="contained"
				startIcon={<PlayArrowIcon />}
				loading={loading}
				onClick={process}
			>
				{loading ? '处理中…' : '添加水印'}
			</Button>
			{result ? (
				<ResultPanel
					result={result}
					sourceUrl={sourceUrl}
					sourceName={file.name}
				/>
			) : null}
		</Stack>
	)
}
