import DarkModeIcon from '@mui/icons-material/DarkMode'
import LightModeIcon from '@mui/icons-material/LightMode'
import {
	Alert,
	AppBar,
	Box,
	Container,
	CssBaseline,
	IconButton,
	Paper,
	Stack,
	Tab,
	Tabs,
	Toolbar,
	Tooltip,
	Typography,
} from '@mui/material'
import type { PaletteMode } from '@mui/material/styles'
import { ThemeProvider } from '@mui/material/styles'
import { useMemo, useState } from 'react'
import BatchDropzone from './components/BatchDropzone'
import ImageDropzone from './components/ImageDropzone'
import { buildTheme } from './theme/theme'
import BatchConvertPanel from './tools/BatchConvertPanel'
import BatchResizePanel from './tools/BatchResizePanel'
import BatchWatermarkPanel from './tools/BatchWatermarkPanel'
import CompressPanel from './tools/CompressPanel'
import ConvertPanel from './tools/ConvertPanel'
import CropPanel from './tools/CropPanel'
import InspectPanel from './tools/InspectPanel'
import ResizePanel from './tools/ResizePanel'
import WatermarkPanel from './tools/WatermarkPanel'

const SECTIONS = [
	{ key: 'tools', label: '图片工具' },
	{ key: 'batch', label: '批量处理' },
] as const

type SectionKey = (typeof SECTIONS)[number]['key']

const TOOLS = [
	{ key: 'compress', label: '压缩', Component: CompressPanel },
	{ key: 'resize', label: '缩放', Component: ResizePanel },
	{ key: 'crop', label: '裁剪', Component: CropPanel },
	{ key: 'convert', label: '转换', Component: ConvertPanel },
	{ key: 'watermark', label: '水印', Component: WatermarkPanel },
	{ key: 'inspect', label: '检查', Component: InspectPanel },
] as const

type ToolKey = (typeof TOOLS)[number]['key']

const BATCH_TOOLS = [
	{ key: 'resize', label: '缩放', Component: BatchResizePanel },
	{ key: 'convert', label: '转换', Component: BatchConvertPanel },
	{ key: 'watermark', label: '水印', Component: BatchWatermarkPanel },
] as const

type BatchToolKey = (typeof BATCH_TOOLS)[number]['key']

export default function App() {
	const [mode, setMode] = useState<PaletteMode>('light')
	const [section, setSection] = useState<SectionKey>('tools')
	const [file, setFile] = useState<File | null>(null)
	const [tool, setTool] = useState<ToolKey>('compress')
	const [batchFiles, setBatchFiles] = useState<File[]>([])
	const [batchTool, setBatchTool] = useState<BatchToolKey>('resize')
	const theme = useMemo(() => buildTheme(mode), [mode])

	const active = TOOLS.find((t) => t.key === tool)
	const ActivePanel = active?.Component ?? CompressPanel
	const activeBatch = BATCH_TOOLS.find((t) => t.key === batchTool)
	const ActiveBatchPanel = activeBatch?.Component ?? BatchResizePanel

	return (
		<ThemeProvider theme={theme}>
			<CssBaseline />
			<AppBar position="static">
				<Toolbar>
					<Typography variant="h6" component="div" sx={{ flexGrow: 1 }}>
						Image Tool Box
					</Typography>
					<Tooltip title={mode === 'light' ? '切换暗黑模式' : '切换明亮模式'}>
						<IconButton
							color="inherit"
							onClick={() => setMode(mode === 'light' ? 'dark' : 'light')}
						>
							{mode === 'light' ? <DarkModeIcon /> : <LightModeIcon />}
						</IconButton>
					</Tooltip>
				</Toolbar>
			</AppBar>
			<Container maxWidth="lg" sx={{ py: 3 }}>
				<Tabs
					value={section}
					onChange={(_, v: string) => setSection(v as SectionKey)}
					sx={{ borderBottom: 1, borderColor: 'divider', mb: 3 }}
				>
					{SECTIONS.map((s) => (
						<Tab key={s.key} value={s.key} label={s.label} />
					))}
				</Tabs>

				{section === 'tools' ? (
					<Stack direction={{ xs: 'column', md: 'row' }} spacing={3}>
						<Box sx={{ flex: 1, minWidth: 0 }}>
							<ImageDropzone file={file} onChange={setFile} />
						</Box>
						<Paper
							sx={{ flex: 1.2, minWidth: 0, p: 2.5, alignSelf: 'flex-start' }}
						>
							<Tabs
								value={tool}
								onChange={(_, v: string) => setTool(v as ToolKey)}
								variant="scrollable"
								scrollButtons="auto"
								sx={{ borderBottom: 1, borderColor: 'divider', mb: 2 }}
							>
								{TOOLS.map((t) => (
									<Tab key={t.key} value={t.key} label={t.label} />
								))}
							</Tabs>
							{file ? (
								<ActivePanel file={file} />
							) : (
								<Alert severity="info">请先选择一张图片</Alert>
							)}
						</Paper>
					</Stack>
				) : (
					<Stack direction={{ xs: 'column', md: 'row' }} spacing={3}>
						<Box sx={{ flex: 1, minWidth: 0 }}>
							<BatchDropzone files={batchFiles} onChange={setBatchFiles} />
						</Box>
						<Paper
							sx={{ flex: 1.2, minWidth: 0, p: 2.5, alignSelf: 'flex-start' }}
						>
							<Tabs
								value={batchTool}
								onChange={(_, v: string) => setBatchTool(v as BatchToolKey)}
								sx={{ borderBottom: 1, borderColor: 'divider', mb: 2 }}
							>
								{BATCH_TOOLS.map((t) => (
									<Tab key={t.key} value={t.key} label={t.label} />
								))}
							</Tabs>
							{batchFiles.length > 0 ? (
								<ActiveBatchPanel files={batchFiles} />
							) : (
								<Alert severity="info">请先选择要批量处理的图片</Alert>
							)}
						</Paper>
					</Stack>
				)}
			</Container>
		</ThemeProvider>
	)
}
