import DarkModeIcon from '@mui/icons-material/DarkMode'
import LightModeIcon from '@mui/icons-material/LightMode'
import SettingsBrightnessIcon from '@mui/icons-material/SettingsBrightness'
import {
	Alert,
	AppBar,
	Box,
	Container,
	CssBaseline,
	Dialog,
	DialogContent,
	IconButton,
	LinearProgress,
	Menu,
	MenuItem,
	Paper,
	Stack,
	Tab,
	Tabs,
	Toolbar,
	Tooltip,
	Typography,
	useMediaQuery,
} from '@mui/material'
import type { PaletteMode } from '@mui/material/styles'
import { ThemeProvider } from '@mui/material/styles'
import { useCallback, useEffect, useMemo, useState } from 'react'
import type { ProcessResult } from './api/client'
import ImageDropzone from './components/ImageDropzone'
import ResultPanel from './components/ResultPanel'
import { buildTheme } from './theme/theme'
import CompressPanel from './tools/CompressPanel'
import ConvertPanel from './tools/ConvertPanel'
import CropPanel from './tools/CropPanel'
import ResizePanel from './tools/ResizePanel'
import WatermarkPanel from './tools/WatermarkPanel'

const TOOLS = [
	{ key: 'compress', label: '压缩', Component: CompressPanel },
	{ key: 'resize', label: '缩放', Component: ResizePanel },
	{ key: 'crop', label: '裁剪', Component: CropPanel },
	{ key: 'convert', label: '转换', Component: ConvertPanel },
	{ key: 'watermark', label: '水印', Component: WatermarkPanel },
] as const

type ToolKey = (typeof TOOLS)[number]['key']
type ThemePreference = PaletteMode | 'system'

const THEME_OPTIONS: { value: ThemePreference; label: string }[] = [
	{ value: 'system', label: '跟随设备' },
	{ value: 'light', label: '明亮模式' },
	{ value: 'dark', label: '暗黑模式' },
]
const THEME_PREFERENCE_STORAGE_KEY = 'itb-theme-preference'

function isThemePreference(value: string | null): value is ThemePreference {
	return THEME_OPTIONS.some((option) => option.value === value)
}

function loadThemePreference(): ThemePreference {
	if (typeof window === 'undefined') {
		return 'system'
	}

	try {
		const value = window.localStorage.getItem(THEME_PREFERENCE_STORAGE_KEY)
		return isThemePreference(value) ? value : 'system'
	} catch {
		return 'system'
	}
}

export default function App() {
	const [themePreference, setThemePreference] =
		useState<ThemePreference>(loadThemePreference)
	const [themeMenuAnchor, setThemeMenuAnchor] = useState<HTMLElement | null>(
		null,
	)
	const prefersDarkMode = useMediaQuery('(prefers-color-scheme: dark)')
	const [file, setFile] = useState<File | null>(null)
	const [tool, setTool] = useState<ToolKey>('compress')
	const [processedResult, setProcessedResult] = useState<ProcessResult | null>(
		null,
	)
	const [processing, setProcessing] = useState(false)
	const mode: PaletteMode =
		themePreference === 'system'
			? prefersDarkMode
				? 'dark'
				: 'light'
			: themePreference
	const theme = useMemo(() => buildTheme(mode), [mode])
	const currentThemeOption = THEME_OPTIONS.find(
		(option) => option.value === themePreference,
	)

	useEffect(() => {
		try {
			window.localStorage.setItem(THEME_PREFERENCE_STORAGE_KEY, themePreference)
		} catch {
			// Storage can be unavailable in private or restricted browsing contexts.
		}
	}, [themePreference])

	const active = TOOLS.find((t) => t.key === tool)
	const ActivePanel = active?.Component ?? CompressPanel
	const handleResult = useCallback((result: ProcessResult | null) => {
		setProcessedResult(result)
	}, [])
	const handleLoadingChange = useCallback((loading: boolean) => {
		setProcessing(loading)
	}, [])
	const activePanel = file ? (
		<ActivePanel
			file={file}
			onResult={handleResult}
			onLoadingChange={handleLoadingChange}
		/>
	) : (
		<Alert severity="info">请先选择一张图片</Alert>
	)

	return (
		<ThemeProvider theme={theme}>
			<CssBaseline />
			<AppBar position="static">
				<Toolbar>
					<Typography variant="h6" component="div" sx={{ flexGrow: 1 }}>
						Image Tool Box
					</Typography>
					<Tooltip title={`主题：${currentThemeOption?.label ?? '跟随设备'}`}>
						<IconButton
							color="inherit"
							aria-label="选择主题"
							aria-controls={themeMenuAnchor ? 'theme-menu' : undefined}
							aria-haspopup="true"
							onClick={(event) => setThemeMenuAnchor(event.currentTarget)}
						>
							{themePreference === 'system' ? (
								<SettingsBrightnessIcon />
							) : mode === 'light' ? (
								<LightModeIcon />
							) : (
								<DarkModeIcon />
							)}
						</IconButton>
					</Tooltip>
					<Menu
						id="theme-menu"
						anchorEl={themeMenuAnchor}
						open={Boolean(themeMenuAnchor)}
						onClose={() => setThemeMenuAnchor(null)}
					>
						{THEME_OPTIONS.map((option) => (
							<MenuItem
								key={option.value}
								selected={themePreference === option.value}
								onClick={() => {
									setThemePreference(option.value)
									setThemeMenuAnchor(null)
								}}
							>
								{option.label}
							</MenuItem>
						))}
					</Menu>
				</Toolbar>
			</AppBar>
			<Container maxWidth="lg" sx={{ py: 3 }}>
				<Tabs
					value={tool}
					onChange={(_, v: string) => {
						setTool(v as ToolKey)
						setProcessedResult(null)
						setProcessing(false)
					}}
					variant="scrollable"
					scrollButtons="auto"
					sx={{ borderBottom: 1, borderColor: 'divider', mb: 3 }}
				>
					{TOOLS.map((item) => (
						<Tab key={item.key} value={item.key} label={item.label} />
					))}
				</Tabs>

				<Stack spacing={3} sx={{ alignItems: 'center' }}>
					<Stack
						direction={{ xs: 'column', md: 'row' }}
						spacing={3}
						sx={{
							width: '100%',
							maxWidth: processedResult ? 1120 : 640,
						}}
					>
						<Box sx={{ flex: 1, minWidth: 0 }}>
							<ImageDropzone
								file={file}
								onChange={(nextFile) => {
									setFile(nextFile)
									setProcessedResult(null)
									setProcessing(false)
								}}
							/>
						</Box>
						{processedResult ? (
							<Box sx={{ flex: 1, minWidth: 0 }}>
								<ResultPanel result={processedResult} />
							</Box>
						) : null}
					</Stack>
					{tool === 'compress' ? (
						<Box sx={{ width: '100%', maxWidth: 760, px: 2.5 }}>
							{activePanel}
						</Box>
					) : (
						<Paper sx={{ width: '100%', maxWidth: 760, p: 2.5 }}>
							{activePanel}
						</Paper>
					)}
				</Stack>
				<Dialog
					open={processing}
					aria-labelledby="compression-progress-title"
					slotProps={{
						paper: { sx: { width: 360, maxWidth: 'calc(100% - 48px)' } },
					}}
				>
					<DialogContent sx={{ px: 3, pt: 2.5, pb: 2 }}>
						<Typography
							id="compression-progress-title"
							variant="subtitle1"
							gutterBottom
						>
							正在压缩图片
						</Typography>
						<Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
							请稍候，完成后将自动更新处理结果。
						</Typography>
						<LinearProgress aria-label="正在压缩图片" />
					</DialogContent>
				</Dialog>
			</Container>
		</ThemeProvider>
	)
}
