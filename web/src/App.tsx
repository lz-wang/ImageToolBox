import DarkModeIcon from '@mui/icons-material/DarkMode'
import LightModeIcon from '@mui/icons-material/LightMode'
import SettingsBrightnessIcon from '@mui/icons-material/SettingsBrightness'
import {
	Alert,
	AppBar,
	Box,
	Container,
	CssBaseline,
	IconButton,
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
import InspectPanel from './tools/InspectPanel'
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
								}}
							/>
						</Box>
						{processedResult ? (
							<Paper sx={{ flex: 1, minWidth: 0, p: 2.5 }}>
								<Typography variant="subtitle2" sx={{ mb: 1 }}>
									处理结果
								</Typography>
								<ResultPanel result={processedResult} />
							</Paper>
						) : null}
					</Stack>
					<Paper sx={{ width: '100%', maxWidth: 760, p: 2.5 }}>
						{file ? (
							<ActivePanel file={file} onResult={handleResult} />
						) : (
							<Alert severity="info">请先选择一张图片</Alert>
						)}
					</Paper>
					{file ? (
						<Box sx={{ width: '100%', maxWidth: 960 }}>
							<InspectPanel file={file} />
						</Box>
					) : null}
				</Stack>
			</Container>
		</ThemeProvider>
	)
}
