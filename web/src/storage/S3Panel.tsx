import DeleteIcon from '@mui/icons-material/Delete'
import DownloadIcon from '@mui/icons-material/Download'
import InfoIcon from '@mui/icons-material/Info'
import RefreshIcon from '@mui/icons-material/Refresh'
import UploadIcon from '@mui/icons-material/Upload'
import {
	Alert,
	Box,
	Button,
	Chip,
	CircularProgress,
	Dialog,
	DialogContent,
	IconButton,
	Stack,
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableRow,
	TextField,
	Tooltip,
	Typography,
} from '@mui/material'
import { useCallback, useEffect, useRef, useState } from 'react'
import type { S3Object, S3ObjectStat, S3Status } from '../api/client'
import {
	deleteS3Object,
	fetchS3ObjectStat,
	fetchS3Objects,
	fetchS3Status,
	s3DownloadUrl,
	uploadS3Object,
} from '../api/client'
import { formatBytes } from '../lib/format'

/** S3 存储面板：状态、列表、上传、下载、删除。凭证从服务端环境变量读取 */
export default function S3Panel() {
	const [status, setStatus] = useState<S3Status | null>(null)
	const [statusError, setStatusError] = useState('')
	const [objects, setObjects] = useState<S3Object[]>([])
	const [listError, setListError] = useState('')
	const [listLoading, setListLoading] = useState(false)
	const [prefix, setPrefix] = useState('')
	const [uploading, setUploading] = useState(false)
	const [uploadError, setUploadError] = useState('')
	const [statKey, setStatKey] = useState('')
	const [stat, setStat] = useState<S3ObjectStat | null>(null)
	const [statError, setStatError] = useState('')
	const [statLoading, setStatLoading] = useState(false)
	const fileInputRef = useRef<HTMLInputElement>(null)

	const refresh = useCallback(async () => {
		setListLoading(true)
		setListError('')
		try {
			setObjects(await fetchS3Objects(prefix))
		} catch (err) {
			setListError(err instanceof Error ? err.message : String(err))
		} finally {
			setListLoading(false)
		}
	}, [prefix])

	useEffect(() => {
		fetchS3Status()
			.then(setStatus)
			.catch((err: unknown) =>
				setStatusError(err instanceof Error ? err.message : String(err)),
			)
	}, [])

	useEffect(() => {
		if (status?.configured) {
			refresh()
		}
	}, [status, refresh])

	const upload = async () => {
		const file = fileInputRef.current?.files?.[0]
		if (!file) {
			setUploadError('请先选择要上传的文件')
			return
		}
		setUploading(true)
		setUploadError('')
		try {
			await uploadS3Object(file, { prefix })
			await refresh()
		} catch (err) {
			setUploadError(err instanceof Error ? err.message : String(err))
		} finally {
			setUploading(false)
			if (fileInputRef.current) {
				fileInputRef.current.value = ''
			}
		}
	}

	const remove = async (key: string) => {
		if (!window.confirm(`确定删除 ${key} 吗？`)) {
			return
		}
		try {
			await deleteS3Object(key)
			await refresh()
		} catch (err) {
			setListError(err instanceof Error ? err.message : String(err))
		}
	}

	const showStat = async (key: string) => {
		setStatLoading(true)
		setStatError('')
		setStat(null)
		setStatKey(key)
		try {
			setStat(await fetchS3ObjectStat(key))
		} catch (err) {
			setStatError(err instanceof Error ? err.message : String(err))
		} finally {
			setStatLoading(false)
		}
	}

	const closeStat = () => {
		setStatKey('')
		setStat(null)
		setStatError('')
	}

	if (statusError) {
		return <Alert severity="error">{statusError}</Alert>
	}
	if (!status) {
		return <CircularProgress size={24} />
	}
	if (!status.configured) {
		return (
			<Alert severity="info">
				S3 未配置。请在启动 itb serve 的环境中设置
				ITB_S3_ENDPOINT、ITB_S3_ACCESS_KEY_ID、
				ITB_S3_SECRET_ACCESS_KEY、ITB_S3_BUCKET（可选 ITB_S3_REGION）后重启。
				Secret 不会进入浏览器。
			</Alert>
		)
	}

	return (
		<Stack spacing={2}>
			<Stack
				direction="row"
				spacing={1}
				useFlexGap
				sx={{ flexWrap: 'wrap', alignItems: 'center' }}
			>
				<Chip size="small" label={`endpoint: ${status.endpoint}`} />
				<Chip size="small" label={`bucket: ${status.bucket}`} />
				<Chip size="small" label={`region: ${status.region}`} />
			</Stack>

			<Stack
				direction={{ xs: 'column', sm: 'row' }}
				spacing={1}
				sx={{ alignItems: 'center' }}
			>
				<TextField
					size="small"
					label="前缀过滤"
					value={prefix}
					onChange={(e) => setPrefix(e.target.value)}
					sx={{ minWidth: 240 }}
				/>
				<Tooltip title="刷新列表">
					<IconButton onClick={refresh} disabled={listLoading}>
						<RefreshIcon />
					</IconButton>
				</Tooltip>
				<Box sx={{ flexGrow: 1 }} />
				<input ref={fileInputRef} type="file" hidden />
				<Button
					variant="contained"
					startIcon={<UploadIcon />}
					loading={uploading}
					disabled={uploading}
					onClick={upload}
				>
					上传到当前前缀
				</Button>
			</Stack>

			{uploadError ? <Alert severity="error">{uploadError}</Alert> : null}
			{listError ? <Alert severity="error">{listError}</Alert> : null}
			{listLoading ? <CircularProgress size={24} /> : null}

			{!listLoading && objects.length === 0 ? (
				<Typography color="text.secondary">没有对象</Typography>
			) : null}

			{objects.length > 0 ? (
				<Table size="small">
					<TableHead>
						<TableRow>
							<TableCell>Key</TableCell>
							<TableCell>大小</TableCell>
							<TableCell>修改时间</TableCell>
							<TableCell align="right">操作</TableCell>
						</TableRow>
					</TableHead>
					<TableBody>
						{objects.map((obj) => (
							<TableRow key={obj.key}>
								<TableCell sx={{ wordBreak: 'break-all' }}>{obj.key}</TableCell>
								<TableCell>{formatBytes(obj.size)}</TableCell>
								<TableCell>
									{obj.last_modified
										? new Date(obj.last_modified).toLocaleString()
										: '-'}
								</TableCell>
								<TableCell align="right">
									<Tooltip title="详情">
										<IconButton size="small" onClick={() => showStat(obj.key)}>
											<InfoIcon fontSize="small" />
										</IconButton>
									</Tooltip>
									<Tooltip title="下载">
										<IconButton
											size="small"
											href={s3DownloadUrl(obj.key)}
											download={obj.key.split('/').pop()}
										>
											<DownloadIcon fontSize="small" />
										</IconButton>
									</Tooltip>
									<Tooltip title="删除">
										<IconButton size="small" onClick={() => remove(obj.key)}>
											<DeleteIcon fontSize="small" />
										</IconButton>
									</Tooltip>
								</TableCell>
							</TableRow>
						))}
					</TableBody>
				</Table>
			) : null}

			<Dialog open={statKey !== ''} onClose={closeStat} fullWidth maxWidth="sm">
				<DialogContent>
					<Typography
						variant="subtitle1"
						sx={{ wordBreak: 'break-all', mb: 1 }}
					>
						{statKey}
					</Typography>
					{statLoading ? <CircularProgress size={20} /> : null}
					{statError ? <Alert severity="error">{statError}</Alert> : null}
					{stat ? (
						<Stack spacing={0.5}>
							<StatRow label="大小" value={formatBytes(stat.size)} />
							<StatRow
								label="修改时间"
								value={
									stat.last_modified
										? new Date(stat.last_modified).toLocaleString()
										: '-'
								}
							/>
							<StatRow label="ETag" value={stat.etag || '-'} />
							{stat.content_type ? (
								<StatRow label="Content-Type" value={stat.content_type} />
							) : null}
							{stat.storage_class ? (
								<StatRow label="存储类型" value={stat.storage_class} />
							) : null}
							{stat.cache_control ? (
								<StatRow label="Cache-Control" value={stat.cache_control} />
							) : null}
							{stat.content_disposition ? (
								<StatRow
									label="Content-Disposition"
									value={stat.content_disposition}
								/>
							) : null}
							{stat.version_id ? (
								<StatRow label="Version ID" value={stat.version_id} />
							) : null}
							{Object.entries(stat.metadata ?? {}).map(([k, v]) => (
								<StatRow key={k} label={`Metadata: ${k}`} value={v} />
							))}
						</Stack>
					) : null}
				</DialogContent>
			</Dialog>
		</Stack>
	)
}

function StatRow({ label, value }: { label: string; value: string }) {
	return (
		<Stack direction="row" spacing={1} sx={{ alignItems: 'baseline' }}>
			<Typography
				variant="body2"
				color="text.secondary"
				sx={{ minWidth: 140, flexShrink: 0 }}
			>
				{label}
			</Typography>
			<Typography variant="body2" sx={{ wordBreak: 'break-all' }}>
				{value}
			</Typography>
		</Stack>
	)
}
