import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import { Alert, Button, LinearProgress, Stack, Typography } from '@mui/material'
import type { ReactNode } from 'react'
import { formatBytes } from '../lib/format'

interface BatchPanelShellProps {
	title: string
	description?: string
	files: File[]
	loading: boolean
	error: string
	result: ReactNode
	onProcess: () => void
	processLabel: string
	children: ReactNode
}

/** 批量面板公共骨架：参数表单 + 进度 + 结果 */
export function BatchPanelShell({
	title,
	description,
	files,
	loading,
	error,
	result,
	onProcess,
	processLabel,
	children,
}: BatchPanelShellProps) {
	return (
		<Stack spacing={2}>
			<Typography variant="h6">{title}</Typography>
			{description ? (
				<Typography variant="body2" color="text.secondary">
					{description}
				</Typography>
			) : null}
			{children}
			{error ? <Alert severity="error">{error}</Alert> : null}
			<Button
				variant="contained"
				startIcon={<PlayArrowIcon />}
				loading={loading}
				disabled={loading || files.length === 0}
				onClick={onProcess}
			>
				{loading
					? '处理中…'
					: `${processLabel}（${files.length} 张 / ${formatBytes(
							files.reduce((sum, f) => sum + f.size, 0),
						)}）`}
			</Button>
			{loading ? <LinearProgress /> : null}
			{result}
		</Stack>
	)
}
