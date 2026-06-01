import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { AppEventProvider } from './events/AppEventProvider';
import { App } from './App';
import { ThemeProvider } from './theme/ThemeContext';
import { AuthProvider } from './auth/AuthContext';
import './styles.css';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <AppEventProvider>
        <ThemeProvider>
          <AuthProvider>
            <App />
          </AuthProvider>
        </ThemeProvider>
      </AppEventProvider>
    </BrowserRouter>
  </StrictMode>
);
