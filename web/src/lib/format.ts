/** 通用格式化工具 */

export function formatBytes(bytes: number): string {
	if (!Number.isFinite(bytes) || bytes < 0) {
		return '-'
	}
	const units = ['B', 'KB', 'MB', 'GB']
	let value = bytes
	let unit = 0
	while (value >= 1024 && unit < units.length - 1) {
		value /= 1024
		unit++
	}
	return unit === 0 ? `${value} B` : `${value.toFixed(1)} ${units[unit]}`
}
