import {
  createContext,
  useContext,
  useState,
  useEffect,
  type ReactNode,
} from "react";
import type { User } from "@/types";
import { t } from "@/lib/i18n";
import { sdk } from "@/lib/sdk";
import {
  getStoredToken,
  getStoredUserJson,
  setStoredUserJson,
  type AuthLoginRequestWire,
} from "@vexgo/sdk";

interface AuthContextType {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (
    email: string,
    password: string,
    captchaData?: { id: string; token: string; x: number; y: number },
  ) => Promise<void>;
  loginWithToken: (token: string) => Promise<void>;
  register: (
    username: string,
    email: string,
    password: string,
    captchaData?: { id: string; token: string; x: number; y: number },
  ) => Promise<void>;
  logout: () => void;
  updateUser: (user: User) => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const token = getStoredToken();
    const savedUser = getStoredUserJson();

    if (token && savedUser) {
      setUser(JSON.parse(savedUser));
      sdk.auth
        .me()
        .then((result) => {
          setUser(result.user as User);
          if (result.user) setStoredUserJson(JSON.stringify(result.user));
        })
        .catch(() => {
          sdk.auth.logout();
          setUser(null);
        })
        .finally(() => {
          setIsLoading(false);
        });
    } else {
      setIsLoading(false);
    }
  }, []);

  const login = async (
    email: string,
    password: string,
    captchaData?: { id: string; token: string; x: number; y: number },
  ) => {
    const requestData: AuthLoginRequestWire = { email, password };
    if (captchaData) {
      requestData.captcha_id = captchaData.id;
      requestData.captcha_token = captchaData.token;
      requestData.captcha_x = captchaData.x;
      requestData.captcha_y = captchaData.y;
    }
    const { user: u, token } = await sdk.auth.login(requestData);
    if (!u || !token) {
      throw new Error(t("loginPage.loginFailed"));
    }
    setUser(u as User);
  };

  const loginWithToken = async (token: string) => {
    const result = await sdk.auth.loginWithToken(token);
    const u = result.user;
    setUser(u as User);
  };

  const register = async (
    username: string,
    email: string,
    password: string,
    captchaData?: { id: string; token: string; x: number; y: number },
  ) => {
    const requestData: {
      username: string;
      email: string;
      password: string;
      captcha_id?: string;
      captcha_token?: string;
      captcha_x?: number;
      captcha_y?: number;
    } = { username, email, password };
    if (captchaData) {
      requestData.captcha_id = captchaData.id;
      requestData.captcha_token = captchaData.token;
      requestData.captcha_x = captchaData.x;
      requestData.captcha_y = captchaData.y;
    }
    try {
      const result = await sdk.auth.register(requestData);
      const { user: u, requires_verification, email_verified } = result;

      if (requires_verification && !email_verified) {
        const error = new Error(
          result.message || t("auth.emailVerificationRequired"),
        ) as Error & {
          requiresVerification: boolean;
          email: string;
          registrationMessage: string;
        };
        error.requiresVerification = true;
        error.email = email;
        error.registrationMessage = result.message || "";
        throw error;
      }

      if (u) {
        setUser(u as User);
        setStoredUserJson(JSON.stringify(u));
      }
    } catch (error: unknown) {
      if (
        error instanceof Error &&
        (error as Error & { requiresVerification?: boolean })
          .requiresVerification
      ) {
        throw error;
      }
      const errorMessage =
        error instanceof Error && error.message
          ? error.message
          : t("auth.registrationFailed");
      throw new Error(errorMessage);
    }
  };

  const logout = () => {
    sdk.auth.logout();
    setUser(null);
  };

  const updateUser = (updatedUser: User) => {
    setUser(updatedUser);
    setStoredUserJson(JSON.stringify(updatedUser));
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated: !!user,
        isLoading,
        login,
        loginWithToken,
        register,
        logout,
        updateUser,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
