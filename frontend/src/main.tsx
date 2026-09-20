import React from 'react';
import ReactDOM from 'react-dom/client';
import { createTheme, CssBaseline, ThemeProvider } from '@mui/material';
import { RouterProvider } from 'react-router-dom';
import { AuthProvider } from './hooks/useAuth';
import { router } from './router';
import './styles.css';

const theme = createTheme({
  palette: {
    mode: 'light', primary: { main: '#116b58' }, secondary: { main: '#b56a16' },
    background: { default: '#f4f6f7', paper: '#ffffff' }, error: { main: '#b8323e' },
  },
  shape: { borderRadius: 6 },
  typography: { fontFamily: 'Inter, "Segoe UI", "PingFang SC", sans-serif', button: { textTransform: 'none', letterSpacing: 0 } },
  components: {
    MuiButton: { defaultProps: { disableElevation: true }, styleOverrides: { root: { minHeight: 38 } } },
    MuiTableCell: { styleOverrides: { head: { fontWeight: 700, color: '#45545d', background: '#eef1f2' } } },
  },
});

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode><ThemeProvider theme={theme}><CssBaseline /><AuthProvider><RouterProvider router={router} future={{ v7_startTransition: true }} /></AuthProvider></ThemeProvider></React.StrictMode>,
);
