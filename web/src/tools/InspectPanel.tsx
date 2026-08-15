import { Alert, Box, Stack, Typography } from '@mui/material'
import { useEffect, useState } from 'react'
import type { InspectResult } from '../api/client'
import { inspectImage } from '../api/client'
import { formatBytes } from '../lib/format'
import type { FileProp } from './types'

export default function InspectPanel({ file }: FileProp) {
	const [loading, setLoading] = useState(false)
	const [error, setError] = useState('')
	const [result, setResult] = useState<InspectResult | null>(null)

	useEffect(() => {
		let cancelled = false

		const load = async () => {
			setLoading(true)
			setError('')
			setResult(null)
			try {
				const nextResult = await inspectImage(file)
				if (!cancelled) {
					setResult(nextResult)
				}
			} catch (err) {
				if (!cancelled) {
					setError(err instanceof Error ? err.message : String(err))
				}
			} finally {
				if (!cancelled) {
					setLoading(false)
				}
			}
		}

		void load()
		return () => {
			cancelled = true
		}
	}, [file])

	return (
		<Stack
			component="section"
			aria-label="图片信息"
			spacing={1.25}
			sx={{ borderTop: 1, borderColor: 'divider', pt: 2 }}
		>
			<Typography
				variant="overline"
				color="text.secondary"
				sx={{ lineHeight: 1 }}
			>
				图片信息
			</Typography>
			{loading ? (
				<Typography variant="body2" color="text.secondary">
					正在读取图片信息…
				</Typography>
			) : null}
			{error ? (
				<Alert severity="error" variant="outlined">
					{error}
				</Alert>
			) : null}
			{result ? <InspectResultView result={result} /> : null}
		</Stack>
	)
}

function InspectResultView({ result }: { result: InspectResult }) {
	const img = result.image
	return (
		<Stack direction={{ xs: 'column', md: 'row' }} spacing={1.5}>
			<Section title="文件">
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
			<Section title="图像">
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
				<Section title="哈希">
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
		<Box
			sx={{
				flex: 1,
				minWidth: 0,
				border: 1,
				borderColor: 'divider',
				borderRadius: 1.5,
				bgcolor: 'action.hover',
				px: 1.5,
				py: 1.25,
			}}
		>
			<Typography
				variant="overline"
				color="text.secondary"
				sx={{ display: 'block', lineHeight: 1.1, mb: 0.75 }}
			>
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
			color="text.secondary"
			sx={
				mono
					? {
							fontFamily: 'monospace',
							fontSize: '0.75rem',
							wordBreak: 'break-all',
						}
					: undefined
			}
		>
			{label}：{value}
		</Typography>
	)
}
