import React, { createContext, useContext, useState, useEffect } from 'react';
import client from '../api/client';

export interface MerchantUser {
  id: string;
  name: string;
  email: string;
  bankName: string;
  accountNumber: string;
  suiAddress: string;
  status: string;
}

interface AuthContextType {
  user: MerchantUser | null;
  token: string | null;
  isLoading: boolean;
  login: (token: string, user: MerchantUser) => void;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<MerchantUser | null>(null);
  const [token, setToken] = useState<string | null>(localStorage.getItem('merchantToken'));
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const initAuth = async () => {
      const storedToken = localStorage.getItem('merchantToken');
      if (storedToken) {
        try {
          // Verify token by fetching 'me'
          const res = await client.get('/merchants/me');
          setUser(res.data);
          setToken(storedToken);
        } catch (error) {
          localStorage.removeItem('merchantToken');
          localStorage.removeItem('merchantUser');
          setToken(null);
          setUser(null);
        }
      }
      setIsLoading(false);
    };

    initAuth();
  }, []);

  const login = (newToken: string, newUser: MerchantUser) => {
    localStorage.setItem('merchantToken', newToken);
    localStorage.setItem('merchantUser', JSON.stringify(newUser));
    setToken(newToken);
    setUser(newUser);
  };

  const logout = () => {
    localStorage.removeItem('merchantToken');
    localStorage.removeItem('merchantUser');
    setToken(null);
    setUser(null);
  };

  return (
    <AuthContext.Provider value={{ user, token, isLoading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};
