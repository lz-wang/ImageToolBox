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
import { useCallback, useEffect, useRef, useState } from 'react'
import type { InspectResult } from '../api/client'
import { inspectImage } from '../api/client'
import { formatBytes } from '../lib/format'

interface ImageLightboxProps {
	file: File
	imageUrl: string
	open: boolean
	onClose: () => void
}

/** 平移边界计算中，图片至少保留在舞台内的可见尺寸（px）。 */
const MIN_VISIBLE_SIZE = 64

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
	const [dragging, setDragging] = useState(false)
	const dragStart = useRef<{
		x: number
		y: number
		originX: number
		originY: number
	} | null>(null)
	const stageRef = useRef<HTMLDivElement>(null)
	const imageRef = useRef<HTMLImageElement>(null)
	const [stageSize, setStageSize] = useState({ width: 0, height: 0 })
	const [imageSize, setImageSize] = useState({ width: 0, height: 0 })

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

	// 跟踪图片布局尺寸（transform 前的基准）与舞台尺寸，作为平移边界依据
	useEffect(() => {
		if (!open) return
		const stage = stageRef.current
		const image = imageRef.current
		if (!stage || !image) return

		const measure = () => {
			setStageSize({ width: stage.clientWidth, height: stage.clientHeight })
			setImageSize({
				width: image.clientWidth || image.naturalWidth,
				height: image.clientHeight || image.naturalHeight,
			})
		}

		measure()
		const observer = new ResizeObserver(measure)
		observer.observe(stage)
		observer.observe(image)
		return () => observer.disconnect()
	}, [open])

	/**
	 * 限制平移，使图片可以自由移动，但至少保留一部分处于舞台内，
	 * 避免图片被完全拖出视口后无法找回。
	 */
	const clampPan = useCallback(
		(pos: { x: number; y: number }) => {
			if (
				stageSize.width === 0 ||
				stageSize.height === 0 ||
				imageSize.width === 0 ||
				imageSize.height === 0
			) {
				return pos
			}

			const rad = ((((rotation % 360) + 360) % 360) * Math.PI) / 180
			const cos = Math.abs(Math.cos(rad))
			const sin = Math.abs(Math.sin(rad))

			const rotatedWidth =
				(imageSize.width * cos + imageSize.height * sin) * scale

			const rotatedHeight =
				(imageSize.width * sin + imageSize.height * cos) * scale

			const minVisibleX = Math.min(
				MIN_VISIBLE_SIZE,
				rotatedWidth,
				stageSize.width,
			)

			const minVisibleY = Math.min(
				MIN_VISIBLE_SIZE,
				rotatedHeight,
				stageSize.height,
			)

			const maxX = Math.max(
				0,
				(stageSize.width + rotatedWidth) / 2 - minVisibleX,
			)

			const maxY = Math.max(
				0,
				(stageSize.height + rotatedHeight) / 2 - minVisibleY,
			)

			return {
				x: Math.min(maxX, Math.max(-maxX, pos.x)),
				y: Math.min(maxY, Math.max(-maxY, pos.y)),
			}
		},
		[imageSize, rotation, scale, stageSize],
	)

	// 缩放、旋转或尺寸变化后，把越界的平移收回允许范围
	useEffect(() => {
		setPosition((current) => clampPan(current))
	}, [clampPan])

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
						overflow: 'hidden',
					},
				},
			}}
		>
			{/* 可变换图片层：裁剪图片 transform 产生的 overflow */}
			<Box
				ref={stageRef}
				sx={{
					position: 'absolute',
					inset: 0,
					overflow: 'hidden',
					display: 'flex',
					alignItems: 'center',
					justifyContent: 'center',
				}}
			>
				<Box
					component="img"
					ref={imageRef}
					src={imageUrl}
					alt={file.name}
					draggable={false}
					onPointerDown={(event) => {
						event.preventDefault()
						event.currentTarget.setPointerCapture(event.pointerId)

						setDragging(true)

						dragStart.current = {
							x: event.clientX,
							y: event.clientY,
							originX: position.x,
							originY: position.y,
						}
					}}
					onPointerMove={(event) => {
						if (!dragStart.current) return
						setPosition(
							clampPan({
								x:
									dragStart.current.originX +
									event.clientX -
									dragStart.current.x,
								y:
									dragStart.current.originY +
									event.clientY -
									dragStart.current.y,
							}),
						)
					}}
					onPointerUp={(event) => {
						dragStart.current = null
						setDragging(false)

						if (event.currentTarget.hasPointerCapture(event.pointerId)) {
							event.currentTarget.releasePointerCapture(event.pointerId)
						}
					}}
					onPointerCancel={() => {
						dragStart.current = null
						setDragging(false)
					}}
					onWheel={(event) => {
						event.preventDefault()
						adjustScale(event.deltaY > 0 ? -0.1 : 0.1)
					}}
					sx={{
						maxWidth: 'calc(100vw - 64px)',
						maxHeight: 'calc(100dvh - 120px)',
						objectFit: 'contain',
						cursor: dragging ? 'grabbing' : 'grab',
						userSelect: 'none',
						touchAction: 'none',
						transform: `translate(${position.x}px, ${position.y}px) scale(${scale}) rotate(${rotation}deg)`,
						transition: dragging ? 'none' : 'transform 160ms ease-out',
					}}
				/>
			</Box>
			<Tooltip title="关闭预览">
				<IconButton
					aria-label="关闭图片预览"
					onClick={onClose}
					sx={{
						position: 'absolute',
						top: 12,
						right: 12,
						zIndex: 2,
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
					zIndex: 2,
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
						zIndex: 2,
						width: 280,
						maxWidth: 'calc(100vw - 48px)',
						maxHeight: 'calc(100dvh - 88px)',
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
