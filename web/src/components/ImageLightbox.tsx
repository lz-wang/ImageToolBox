import CloseIcon from '@mui/icons-material/Close'
import InfoOutlinedIcon from '@mui/icons-material/InfoOutlined'
import RestartAltIcon from '@mui/icons-material/RestartAlt'
import RotateLeftIcon from '@mui/icons-material/RotateLeft'
import RotateRightIcon from '@mui/icons-material/RotateRight'
import ZoomInIcon from '@mui/icons-material/ZoomIn'
import ZoomOutIcon from '@mui/icons-material/ZoomOut'
import {
	Box,
	Dialog,
	IconButton,
	Stack,
	Tooltip,
	Typography,
} from '@mui/material'
import { alpha } from '@mui/material/styles'
import { useEffect, useRef, useState } from 'react'
import type { InspectResult } from '../api/client'
import { inspectImage } from '../api/client'
import { formatBytes } from '../lib/format'

interface ImageLightboxProps {
	file: File
	imageUrl: string
	open: boolean
	onClose: () => void
}

/** 原图与处理结果共用的全屏图片预览。 */
export default function ImageLightbox({
	file,
	imageUrl,
	open,
	onClose,
}: ImageLightboxProps) {
	const [inspectResult, setInspectResult] = useState<InspectResult | null>(null)
	const [inspectLoading, setInspectLoading] = useState(false)
	const [inspectError, setInspectError] = useState('')
	const [scale, setScale] = useState(1)
	const [rotation, setRotation] = useState(0)
	const [position, setPosition] = useState({ x: 0, y: 0 })
	const [showInfo, setShowInfo] = useState(false)
	const dragStart = useRef<{
		x: number
		y: number
		originX: number
		originY: number
	} | null>(null)

	useEffect(() => {
		if (!open) return

		let cancelled = false
		setInspectLoading(true)
		setInspectError('')
		setInspectResult(null)
		void inspectImage(file)
			.then((result) => {
				if (!cancelled) setInspectResult(result)
			})
			.catch((error) => {
				if (!cancelled) {
					setInspectError(
						error instanceof Error ? error.message : String(error),
					)
				}
			})
			.finally(() => {
				if (!cancelled) setInspectLoading(false)
			})

		return () => {
			cancelled = true
		}
	}, [file, open])

	useEffect(() => {
		if (!open) return
		setScale(1)
		setRotation(0)
		setPosition({ x: 0, y: 0 })
		setShowInfo(false)
	}, [open])

	const adjustScale = (amount: number) => {
		setScale((current) => Math.min(5, Math.max(0.25, current + amount)))
	}

	const resetView = () => {
		setScale(1)
		setRotation(0)
		setPosition({ x: 0, y: 0 })
	}

	return (
		<Dialog
			fullScreen
			open={open}
			onClose={onClose}
			aria-label="图片预览"
			slotProps={{
				paper: {
					sx: {
						bgcolor: 'rgba(0, 0, 0, 0.92)',
						m: 0,
						position: 'relative',
						display: 'flex',
						alignItems: 'center',
						justifyContent: 'center',
					},
				},
			}}
		>
			<Box
				component="img"
				src={imageUrl}
				alt={file.name}
				draggable={false}
				onPointerDown={(event) => {
					event.currentTarget.setPointerCapture(event.pointerId)
					dragStart.current = {
						x: event.clientX,
						y: event.clientY,
						originX: position.x,
						originY: position.y,
					}
				}}
				onPointerMove={(event) => {
					if (!dragStart.current) return
					setPosition({
						x: dragStart.current.originX + event.clientX - dragStart.current.x,
						y: dragStart.current.originY + event.clientY - dragStart.current.y,
					})
				}}
				onPointerUp={(event) => {
					dragStart.current = null
					event.currentTarget.releasePointerCapture(event.pointerId)
				}}
				onPointerCancel={() => {
					dragStart.current = null
				}}
				onWheel={(event) => {
					event.preventDefault()
					adjustScale(event.deltaY > 0 ? -0.1 : 0.1)
				}}
				sx={{
					maxWidth: 'calc(100vw - 64px)',
					maxHeight: 'calc(100vh - 120px)',
					m: 'auto',
					objectFit: 'contain',
					cursor: 'grab',
					userSelect: 'none',
					touchAction: 'none',
					transform: `translate(${position.x}px, ${position.y}px) scale(${scale}) rotate(${rotation}deg)`,
					transition: dragStart.current ? 'none' : 'transform 160ms ease-out',
				}}
			/>
			<Tooltip title="关闭预览">
				<IconButton
					aria-label="关闭图片预览"
					onClick={onClose}
					sx={{
						position: 'absolute',
						top: 12,
						right: 12,
						color: 'text.primary',
					}}
				>
					<CloseIcon />
				</IconButton>
			</Tooltip>
			<Box
				role="toolbar"
				aria-label="图片预览控制"
				sx={(theme) => ({
					position: 'absolute',
					bottom: 24,
					left: '50%',
					transform: 'translateX(-50%)',
					display: 'flex',
					gap: 0.5,
					p: 0.5,
					borderRadius: 99,
					bgcolor: alpha(theme.palette.background.paper, 0.86),
					backdropFilter: 'blur(12px)',
					boxShadow: 4,
				})}
			>
				<Tooltip title="缩小">
					<IconButton aria-label="缩小图片" onClick={() => adjustScale(-0.25)}>
						<ZoomOutIcon />
					</IconButton>
				</Tooltip>
				<Tooltip title="放大">
					<IconButton aria-label="放大图片" onClick={() => adjustScale(0.25)}>
						<ZoomInIcon />
					</IconButton>
				</Tooltip>
				<Tooltip title="逆时针旋转">
					<IconButton
						aria-label="逆时针旋转图片"
						onClick={() => setRotation((current) => current - 90)}
					>
						<RotateLeftIcon />
					</IconButton>
				</Tooltip>
				<Tooltip title="顺时针旋转">
					<IconButton
						aria-label="顺时针旋转图片"
						onClick={() => setRotation((current) => current + 90)}
					>
						<RotateRightIcon />
					</IconButton>
				</Tooltip>
				<Tooltip title="恢复原始视图">
					<IconButton aria-label="恢复原始图片视图" onClick={resetView}>
						<RestartAltIcon />
					</IconButton>
				</Tooltip>
				<Tooltip title={showInfo ? '隐藏图片信息' : '显示图片信息'}>
					<IconButton
						aria-label={showInfo ? '隐藏图片信息' : '显示图片信息'}
						aria-pressed={showInfo}
						onClick={() => setShowInfo((current) => !current)}
					>
						<InfoOutlinedIcon />
					</IconButton>
				</Tooltip>
			</Box>
			{showInfo ? (
				<Box
					sx={(theme) => ({
						position: 'absolute',
						top: 64,
						right: 24,
						width: 280,
						maxWidth: 'calc(100vw - 48px)',
						maxHeight: 'calc(100vh - 88px)',
						overflowY: 'auto',
						p: 2,
						borderRadius: 2,
						bgcolor: alpha(theme.palette.background.paper, 0.86),
						backdropFilter: 'blur(12px)',
						boxShadow: 4,
					})}
				>
					<Typography variant="subtitle2" sx={{ mb: 1.25 }}>
						图片信息
					</Typography>
					{inspectLoading ? (
						<Typography variant="body2" color="text.secondary">
							正在读取图片信息…
						</Typography>
					) : null}
					{inspectError ? (
						<Typography variant="body2" color="error">
							{inspectError}
						</Typography>
					) : null}
					{inspectResult ? <InspectDetails result={inspectResult} /> : null}
				</Box>
			) : null}
		</Dialog>
	)
}

function InspectDetails({ result }: { result: InspectResult }) {
	return (
		<Stack spacing={0.75}>
			<LightboxInfo label="文件名" value={result.file.name} />
			<LightboxInfo label="大小" value={formatBytes(result.file.size_bytes)} />
			<LightboxInfo label="MIME" value={result.file.mime_type ?? '-'} />
			{result.image ? (
				<>
					<LightboxInfo
						label="格式"
						value={result.image.format.toUpperCase()}
					/>
					<LightboxInfo
						label="尺寸"
						value={`${result.image.width}×${result.image.height}`}
					/>
					<LightboxInfo label="宽高比" value={result.image.aspect_ratio} />
					<LightboxInfo
						label="像素"
						value={`${result.image.megapixels.toFixed(2)} MP`}
					/>
					<LightboxInfo
						label="透明通道"
						value={result.image.has_alpha ? '有' : '无'}
					/>
				</>
			) : null}
			{result.detail ? (
				<LightboxInfo
					label="扩展名匹配"
					value={result.detail.extension_matches_format ? '匹配' : '不匹配'}
				/>
			) : null}
			{result.hashes ? (
				<>
					<LightboxInfo label="SHA256" value={result.hashes.sha256} mono />
					<LightboxInfo label="MD5" value={result.hashes.md5} mono />
				</>
			) : null}
		</Stack>
	)
}

function LightboxInfo({
	label,
	value,
	mono = false,
}: {
	label: string
	value: string
	mono?: boolean
}) {
	return (
		<Typography
			variant="body2"
			sx={{
				overflowWrap: 'anywhere',
				fontFamily: mono ? 'monospace' : undefined,
			}}
		>
			<Box component="span" sx={{ color: 'text.secondary' }}>
				{label}:
			</Box>
			{value}
		</Typography>
	)
}
