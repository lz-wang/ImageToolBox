import { describe, expect, it } from 'vitest'
import { formatBytes } from './format'

describe('formatBytes', () => {
	it('格式化常见单位', () => {
		expect(formatBytes(0)).toBe('0 B')
		expect(formatBytes(512)).toBe('512 B')
		expect(formatBytes(1024)).toBe('1.0 KB')
		expect(formatBytes(1536)).toBe('1.5 KB')
		expect(formatBytes(1024 * 1024)).toBe('1.0 MB')
	})

	it('非法输入返回占位符', () => {
		expect(formatBytes(Number.NaN)).toBe('-')
		expect(formatBytes(-1)).toBe('-')
	})
})
