import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
	plugins: [react()],
	server: {
		// 开发模式下把 API 代理到本地 itb serve，避免跨域
		proxy: {
			'/api': 'http://127.0.0.1:8080',
		},
	},
})
