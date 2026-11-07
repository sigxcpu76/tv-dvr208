import React from 'react';
import ReactDOM from 'react-dom/client';
import { App } from './App';
import { HttpDvrAdapter } from './adapters/api/HttpDvrAdapter';
import { DvrProvider } from './application/DvrContext';
import './styles.css';

const adapter = new HttpDvrAdapter('');

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <DvrProvider adapter={adapter}>
      <App />
    </DvrProvider>
  </React.StrictMode>
);
