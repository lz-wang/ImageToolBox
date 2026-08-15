import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import UploadIcon from '@mui/icons-material/Upload'
import {
	Alert,
	Box,
	Button,
	Stack,
	TextField,
	Tooltip,
	Typography,
} from '@mui/material'
import { useRef, useState } from 'react'
import type { LskyUploadResult } from '../api/client'
import { uploadLskyImage } from '../api/client'

/** LskyPro 图床面板：上传后提供 Copy URL / Copy Markdown */
export default function LskyPanel() {
	const [strategyId, setStrategyId] = useState('')
	const [uploading, setUploading] = useState(false)
	const [error, setError] = useState('')
	const [result, setResult] = useState<LskyUploadResult | null>(null)
	const [copied, setCopied] = useState('')
	const fileInputRef = useRef<HTMLInputElement>(null)

	const upload = async () => {
		const file = fileInputRef.current?.files?.[0]
		if (!file) {
			setError('请先选择要上传的图片')
			return
		}
		setUploading(true)
		setError('')
		setResult(null)
		try {
			setResult(await uploadLskyImage(file, Number(strategyId) || 0))
		} catch (err) {
			setError(err instanceof Error ? err.message : String(err))
		} finally {
			setUploading(false)
			if (fileInputRef.current) {
				fileInputRef.current.value = ''
			}
		}
	}

	const copy = async (text: string, label: string) => {
		await navigator.clipboard.writeText(text)
		setCopied(label)
		setTimeout(() => setCopied(''), 1500)
	}

	return (
		<Stack spacing={2}>
			<Typography variant="body2" color="text.secondary">
				上传图片到 LskyPro 图床。请在服务端设置 ITB_LSKY_URL 与
				ITB_LSKY_TOKEN，Token 不会进入浏览器。
			</Typography>
			<input ref={fileInputRef} type="file" accept="image/*" hidden />
			<Stack
				direction={{ xs: 'column', sm: 'row' }}
				spacing={1}
				sx={{ alignItems: 'center' }}
			>
				<TextField
					size="small"
					label="存储策略 ID（可选）"
					value={strategyId}
					onChange={(e) => setStrategyId(e.target.value)}
					sx={{ minWidth: 200 }}
				/>
				<Box sx={{ flexGrow: 1 }} />
				<Button
					variant="contained"
					startIcon={<UploadIcon />}
					loading={uploading}
					disabled={uploading}
					onClick={upload}
				>
					上传
				</Button>
			</Stack>
			{error ? <Alert severity="error">{error}</Alert> : null}
			{result ? (
				<Stack spacing={1}>
					<Typography variant="subtitle2">上传成功</Typography>
					<Typography variant="body2" sx={{ wordBreak: 'break-all' }}>
						{result.url}
					</Typography>
					<Stack direction="row" spacing={1}>
						<Tooltip title={copied === 'url' ? '已复制' : '复制 URL'}>
							<Button
								size="small"
								variant="outlined"
								startIcon={<ContentCopyIcon />}
								onClick={() => copy(result.url, 'url')}
							>
								Copy URL
							</Button>
						</Tooltip>
						<Tooltip title={copied === 'markdown' ? '已复制' : '复制 Markdown'}>
							<Button
								size="small"
								variant="outlined"
								startIcon={<ContentCopyIcon />}
								onClick={() => copy(result.markdown, 'markdown')}
							>
								Copy Markdown
							</Button>
						</Tooltip>
					</Stack>
				</Stack>
			) : null}
		</Stack>
	)
}
