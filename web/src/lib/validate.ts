/** 上传文件校验 */

const IMAGE_EXTENSIONS = /\.(jpe?g|png|webp|gif|svg|bmp|tiff?)$/i

/** 判断是否为可接受的图片文件：优先看 MIME，其次看扩展名（部分平台拖拽无 MIME） */
export function isImageFile(file: { type: string; name?: string }): boolean {
	if (file.type.startsWith('image/')) {
		return true
	}
	return IMAGE_EXTENSIONS.test(file.name ?? '')
}
