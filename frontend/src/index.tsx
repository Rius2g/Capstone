// index.tsx
import React from 'react';
import { createRoot } from 'react-dom/client'; // Updated import
import './index.css';
import App from './App';
import { Auth0Provider } from '@auth0/auth0-react';

const domain = "dev-jy5tpcxmm6x8mhg2.us.auth0.com";
const clientId = "gXrbPWJTrnVNAnKGhG03YcZUgbwIDBTS";

const container = document.getElementById('root');
const root = createRoot(container!); // Use createRoot instead of render

root.render(
  <Auth0Provider
    domain={domain}
    clientId={clientId}
  >
    <App />
  </Auth0Provider>
);
