// Header.tsx
import React from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import './Header.css'; // Import the CSS file for styling

export const Header: React.FC = () => {
  const { loginWithRedirect, logout, isAuthenticated, user } = useAuth0();

  const handleLogout = () => {
    // Perform the logout operation
    logout();
    // Manually redirect to the desired page
    window.location.href = window.location.origin;
  };

  return (
    <header className="header">
      <div className="welcome">
        {isAuthenticated && <span>Welcome, {user?.name || 'User'}</span>}
      </div>
      <div className="auth-button">
        {!isAuthenticated ? (
          <button className="button" onClick={() => loginWithRedirect()}>
            Login
          </button>
        ) : (
          <button className="button" onClick={handleLogout}>
            Logout
          </button>
        )}
      </div>
    </header>
  );
};

export default Header;
