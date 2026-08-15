import { createTheme } from '@mui/material/styles'

/** MUI default theme，少量定制；mode 控制明暗 */
export function buildTheme(mode: 'light' | 'dark') {
	return createTheme({
		palette: {
			mode,
		},
	})
}
