import DownloadIcon from '@mui/icons-material/Download'
import { Box, Button, Divider, Stack } from '@mui/material'
import type { ProcessResult } from '../api/client'
import BeforeAfter from './BeforeAfter'

interface ResultPanelProps {
	result: ProcessResult
	/** 原图 object URL，提供时展示 Before / After 对比 */
	sourceUrl?: string
	sourceName?: string
}

/** 处理结果展示：Before/After 对比（或单图预览）+ 下载 */
export default function ResultPanel({
	result,
	sourceUrl,
	sourceName,
}: ResultPanelProps) {
	return (
		<Stack spacing={2} divider={<Divider />}>
			{sourceUrl ? (
				<BeforeAfter
					sourceUrl={sourceUrl}
					sourceName={sourceName ?? '原图'}
					inputSize={result.inputSize}
					outputUrl={result.url}
					outputName={result.filename}
					outputSize={result.outputSize}
				/>
			) : (
				<Box
					component="img"
					src={result.url}
					alt={result.filename}
					sx={{
						maxWidth: '100%',
						maxHeight: 360,
						alignSelf: 'flex-start',
						borderRadius: 1,
						border: '1px solid',
						borderColor: 'divider',
					}}
				/>
			)}
			<Button
				variant="contained"
				size="small"
				startIcon={<DownloadIcon />}
				href={result.url}
				download={result.filename}
				sx={{ alignSelf: 'flex-start' }}
			>
				下载 {result.filename}
			</Button>
		</Stack>
	)
}
