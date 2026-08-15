import type { ProcessResult } from '../api/client'

export interface FileProp {
	file: File
	onResult?: (result: ProcessResult | null) => void
}

export interface FilesProp {
	files: File[]
}
