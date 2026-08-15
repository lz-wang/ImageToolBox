import DownloadIcon from '@mui/icons-material/Download'
import { Box, IconButton, Paper, Tooltip } from '@mui/material'
import type { ProcessResult } from '../api/client'

interface ResultPanelProps {
	result: ProcessResult
}

/** 处理结果卡片：与原图卡片使用相同的展示约束。 */
export default function ResultPanel({ result }: ResultPanelProps) {
	return (
		<Paper
			elevation={1}
			sx={{
				width: 520,
				height: 520,
				maxWidth: '100%',
				boxSizing: 'border-box',
				borderRadius: 2,
				p: 2,
				position: 'relative',
				display: 'flex',
				alignItems: 'center',
				justifyContent: 'center',
			}}
		>
			<Box
				component="img"
				src={result.url}
				alt={result.filename}
				sx={{
					maxWidth: '100%',
					maxHeight: '100%',
					objectFit: 'contain',
					borderRadius: 1,
				}}
			/>
			<Tooltip title="下载图片">
				<IconButton
					size="small"
					aria-label="下载图片"
					href={result.url}
					download={result.filename}
					sx={{
						position: 'absolute',
						right: 16,
						bottom: 16,
						color: 'primary.main',
						'&:hover': { bgcolor: 'transparent', color: 'primary.dark' },
					}}
				>
					<DownloadIcon fontSize="small" />
				</IconButton>
			</Tooltip>
		</Paper>
	)
}
