import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { AppEventProvider } from './events/AppEventProvider';
import { App } from './App';
import { ThemeProvider } from './theme/ThemeContext';
import './styles.css';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter basename="/app">
      <AppEventProvider>
        <ThemeProvider>
          <App />
        </ThemeProvider>
      </AppEventProvider>
    </BrowserRouter>
  </StrictMode>
);
