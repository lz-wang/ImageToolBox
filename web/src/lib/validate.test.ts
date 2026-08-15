import { describe, expect, it } from 'vitest'
import { isImageFile } from './validate'

describe('isImageFile', () => {
	it('接受带 image MIME 的文件', () => {
		expect(isImageFile({ type: 'image/png', name: 'a.bin' })).toBe(true)
		expect(isImageFile({ type: 'image/jpeg', name: 'photo.jpg' })).toBe(true)
	})

	it('MIME 缺失时按扩展名判断', () => {
		expect(isImageFile({ type: '', name: 'photo.jpg' })).toBe(true)
		expect(isImageFile({ type: '', name: 'photo.PNG' })).toBe(true)
		expect(isImageFile({ type: '', name: 'pic.webp' })).toBe(true)
	})

	it('拒绝非图片文件', () => {
		expect(isImageFile({ type: 'text/plain', name: 'note.txt' })).toBe(false)
		expect(isImageFile({ type: '', name: 'archive.zip' })).toBe(false)
		expect(isImageFile({ type: '', name: 'noext' })).toBe(false)
	})
})
