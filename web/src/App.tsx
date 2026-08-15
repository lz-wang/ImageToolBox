import DarkModeIcon from '@mui/icons-material/DarkMode'
import LightModeIcon from '@mui/icons-material/LightMode'
import {
	Alert,
	AppBar,
	Box,
	Button,
	IconButton,
	Paper,
	Stack,
	Toolbar,
	Tooltip,
	Typography,
} from '@mui/material'
import CssBaseline from '@mui/material/CssBaseline'
import type { PaletteMode } from '@mui/material/styles'
import { ThemeProvider } from '@mui/material/styles'
import { useMemo, useState } from 'react'
import { fetchHealth } from './api/client'
import { buildTheme } from './theme/theme'

export default function App() {
	const [mode, setMode] = useState<PaletteMode>('light')
	const theme = useMemo(() => buildTheme(mode), [mode])

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
			<Box sx={{ maxWidth: 960, mx: 'auto', p: 2 }}>
				<Paper sx={{ p: 3 }}>
					<Stack spacing={2}>
						<Typography variant="h5">WebUI 骨架已就绪</Typography>
						<Typography variant="body2" color="text.secondary">
							图片工具面板（压缩、缩放、裁剪、格式转换、水印、元数据检查）将在后续阶段加入。
						</Typography>
						<HealthIndicator />
					</Stack>
				</Paper>
			</Box>
		</ThemeProvider>
	)
}

function HealthIndicator() {
	const [status, setStatus] = useState<'unknown' | 'ok' | 'error'>('unknown')
	const [message, setMessage] = useState('')

	const check = async () => {
		try {
			const result = await fetchHealth()
			setStatus(result.status === 'ok' ? 'ok' : 'error')
			setMessage(result.status)
		} catch (err) {
			setStatus('error')
			setMessage(err instanceof Error ? err.message : String(err))
		}
	}

	return (
		<Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
			<Button variant="outlined" onClick={check}>
				检查后端健康状态
			</Button>
			{status === 'ok' && <Alert severity="success">API 正常</Alert>}
			{status === 'error' && <Alert severity="error">{message}</Alert>}
		</Stack>
	)
}
