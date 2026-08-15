import DownloadIcon from '@mui/icons-material/Download'
import { Button, Chip, Divider, Stack, Typography } from '@mui/material'
import type { BatchProcessResult } from '../api/client'
import { formatBytes } from '../lib/format'

/** 批量处理结果：统计 + zip 下载 */
export default function BatchResultPanel({
	result,
}: {
	result: BatchProcessResult
}) {
	return (
		<Stack spacing={2} divider={<Divider />}>
			<Stack
				direction="row"
				spacing={1}
				useFlexGap
				sx={{ flexWrap: 'wrap', alignItems: 'center' }}
			>
				<Chip size="small" color="success" label={`成功 ${result.success}`} />
				{result.skipped > 0 ? (
					<Chip size="small" label={`跳过 ${result.skipped}`} />
				) : null}
				{result.failed > 0 ? (
					<Chip size="small" color="error" label={`失败 ${result.failed}`} />
				) : null}
				<Typography variant="body2" color="text.secondary">
					{formatBytes(result.blob.size)}
				</Typography>
			</Stack>
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
