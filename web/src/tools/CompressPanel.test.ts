import { describe, expect, it } from 'vitest'
import { compressedFilename } from './CompressPanel'

describe('compressedFilename', () => {
	it('adds the compression quality before a simple extension', () => {
		expect(compressedFilename('photo.jpg', 80)).toBe('photo--q80.jpg')
	})

	it('preserves multi-part base names and extensions', () => {
		expect(compressedFilename('photo.original.png', 1)).toBe(
			'photo.original--q1.png',
		)
	})

	it('handles files without an extension', () => {
		expect(compressedFilename('photo', 100)).toBe('photo--q100')
	})
})
