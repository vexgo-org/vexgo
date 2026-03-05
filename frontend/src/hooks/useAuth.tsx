import { createContext, useContext, useState, useEffect, type ReactNode } from 'react';
import type { User } from '@/types';
import { authApi } from '@/lib/api';

interface AuthContextType {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (email: string, password: string, captchaData?: { id: string; token: string; x: number }) => Promise<void>;
  register: (username: string, email: string, password: string, captchaData?: { id: string; token: string; x: number }) => Promise<void>;
  logout: () => void;
  updateUser: (user: User) => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    // 检查本地存储的token
    const token = localStorage.getItem('token');
    const savedUser = localStorage.getItem('user');
    
    if (token && savedUser) {
      setUser(JSON.parse(savedUser));
      // 验证token有效性
      authApi.getMe()
        .then(response => {
          setUser(response.data.user);
          localStorage.setItem('user', JSON.stringify(response.data.user));
        })
        .catch(() => {
          // Token无效，清除存储
          localStorage.removeItem('token');
          localStorage.removeItem('user');
          setUser(null);
        })
        .finally(() => {
          setIsLoading(false);
        });
    } else {
      setIsLoading(false);
    }
  }, []);

  const login = async (email: string, password: string, captchaData?: { id: string; token: string; x: number }) => {
    const requestData: any = { email, password };
    
    // 如果提供了验证码数据，添加到请求中
    if (captchaData) {
      requestData.captcha_id = captchaData.id;
      requestData.captcha_token = captchaData.token;
      requestData.captcha_x = captchaData.x;
    }
    
    const response = await authApi.login(requestData);
    const { user, token } = response.data;
    
    if (token) {
      localStorage.setItem('token', token);
      localStorage.setItem('user', JSON.stringify(user));
      setUser(user);
    }
  };

  const register = async (username: string, email: string, password: string, captchaData?: { id: string; token: string; x: number }) => {
    const requestData: any = { username, email, password };
    
    // 如果提供了验证码数据，添加到请求中
    if (captchaData) {
      requestData.captcha_id = captchaData.id;
      requestData.captcha_token = captchaData.token;
      requestData.captcha_x = captchaData.x;
    }
    
    const response = await authApi.register(requestData);
    const { user, token, requires_verification, email_verified } = response.data;
    
    // 如果 SMTP 启用且需要验证，则不自动登录，只保存用户信息到状态（但不设置 token）
    if (requires_verification && !email_verified) {
      setUser(user);
      // 抛出特定错误，让前端页面可以显示验证提示
      const error = new Error(response.data.message || '请先验证您的邮箱地址');
      (error as any).requiresVerification = true;
      (error as any).email = email;
      throw error;
    }
    
    // 正常登录（SMTP 未启用或已验证）
    if (token) {
      localStorage.setItem('token', token);
      localStorage.setItem('user', JSON.stringify(user));
      setUser(user);
    } else {
      setUser(user);
    }
  };

  const logout = () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    setUser(null);
  };

  const updateUser = (updatedUser: User) => {
    setUser(updatedUser);
    localStorage.setItem('user', JSON.stringify(updatedUser));
  };

  return (
    <AuthContext.Provider value={{
      user,
      isAuthenticated: !!user,
      isLoading,
      login,
      register,
      logout,
      updateUser
    }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}