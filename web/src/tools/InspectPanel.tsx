import SearchIcon from '@mui/icons-material/Search'
import { Alert, Box, Button, Divider, Stack, Typography } from '@mui/material'
import { useState } from 'react'
import type { InspectResult } from '../api/client'
import { inspectImage } from '../api/client'
import { formatBytes } from '../lib/format'
import type { FileProp } from './types'

export default function InspectPanel({ file }: FileProp) {
	const [loading, setLoading] = useState(false)
	const [error, setError] = useState('')
	const [result, setResult] = useState<InspectResult | null>(null)

	const process = async () => {
		setLoading(true)
		setError('')
		setResult(null)
		try {
			setResult(await inspectImage(file))
		} catch (err) {
			setError(err instanceof Error ? err.message : String(err))
		} finally {
			setLoading(false)
		}
	}

	return (
		<Stack spacing={2}>
			<Typography variant="h6">元数据检查 Inspect</Typography>
			<Button
				variant="contained"
				startIcon={<SearchIcon />}
				loading={loading}
				disabled={loading}
				onClick={process}
			>
				{loading ? '检查中…' : '开始检查'}
			</Button>
			{error ? <Alert severity="error">{error}</Alert> : null}
			{result ? <InspectResultView result={result} /> : null}
		</Stack>
	)
}

function InspectResultView({ result }: { result: InspectResult }) {
	const img = result.image
	return (
		<Stack divider={<Divider />} spacing={2}>
			<Section title="File">
				<Item label="文件名" value={result.file.name} />
				<Item label="大小" value={formatBytes(result.file.size_bytes)} />
				<Item label="MIME" value={result.file.mime_type ?? '-'} />
				{result.detail ? (
					<Item
						label="扩展名匹配"
						value={result.detail.extension_matches_format ? '匹配' : '不匹配'}
					/>
				) : null}
			</Section>
			<Section title="Image">
				{img ? (
					<>
						<Item label="格式" value={img.format} />
						<Item label="尺寸" value={`${img.width}×${img.height}`} />
						<Item label="宽高比" value={img.aspect_ratio} />
						<Item label="像素" value={`${img.megapixels.toFixed(2)} MP`} />
						<Item label="透明通道" value={img.has_alpha ? '有' : '无'} />
					</>
				) : (
					<Typography color="text.secondary">
						解析失败：{result.error?.message ?? '未知错误'}
					</Typography>
				)}
			</Section>
			{result.hashes ? (
				<Section title="Hashes">
					<Item label="SHA256" value={result.hashes.sha256} mono />
					<Item label="MD5" value={result.hashes.md5} mono />
				</Section>
			) : null}
		</Stack>
	)
}

function Section({
	title,
	children,
}: {
	title: string
	children: React.ReactNode
}) {
	return (
		<Box>
			<Typography variant="subtitle2" gutterBottom>
				{title}
			</Typography>
			<Stack spacing={0.5}>{children}</Stack>
		</Box>
	)
}

function Item({
	label,
	value,
	mono,
}: {
	label: string
	value: string
	mono?: boolean
}) {
	return (
		<Typography
			variant="body2"
			sx={
				mono ? { fontFamily: 'monospace', wordBreak: 'break-all' } : undefined
			}
		>
			{label}：{value}
		</Typography>
	)
}
