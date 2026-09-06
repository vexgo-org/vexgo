import {
  createContext,
  useContext,
  useState,
  useEffect,
  type ReactNode,
} from "react";
import type { User } from "@/types";
import { t } from "@/lib/i18n";
import { getVexGoAPI } from "@/api/generated/endpoints";
import type { PostAuthLoginBody } from "@/api/generated/model";

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
    const token = localStorage.getItem("token");
    const savedUser = localStorage.getItem("user");

    if (token && savedUser) {
      setUser(JSON.parse(savedUser));
      getVexGoAPI()
        .getAuthMe()
        .then((response) => {
          setUser(response.data.user as User);
          localStorage.setItem("user", JSON.stringify(response.data.user));
        })
        .catch(() => {
          localStorage.removeItem("token");
          localStorage.removeItem("user");
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
    const requestData: PostAuthLoginBody = { email, password };
    if (captchaData) {
      requestData.captcha_id = captchaData.id;
      requestData.captcha_token = captchaData.token;
      requestData.captcha_x = captchaData.x;
      requestData.captcha_y = captchaData.y;
    }
    const response = await getVexGoAPI().postAuthLogin(requestData);
    const { user: u, token } = response.data;
    if (!u || !token) {
      throw new Error(t("loginPage.loginFailed"));
    }
    localStorage.setItem("token", token);
    localStorage.setItem("user", JSON.stringify(u));
    setUser(u as User);
  };

  const loginWithToken = async (token: string) => {
    localStorage.setItem("token", token);
    const response = await getVexGoAPI().getAuthMe();
    const u = response.data.user;
    localStorage.setItem("user", JSON.stringify(u));
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
      const response = await getVexGoAPI().postAuthRegister(requestData);
      const { user: u, requires_verification, email_verified } = response.data;

      if (requires_verification && !email_verified) {
        const error = new Error(
          response.data.message || t("auth.emailVerificationRequired"),
        ) as Error & {
          requiresVerification: boolean;
          email: string;
          registrationMessage: string;
        };
        error.requiresVerification = true;
        error.email = email;
        error.registrationMessage = response.data.message || "";
        throw error;
      }

      if (u) {
        setUser(u as User);
        localStorage.setItem("user", JSON.stringify(u));
      }
    } catch (error: unknown) {
      if (
        error instanceof Error &&
        (error as Error & { requiresVerification?: boolean })
          .requiresVerification
      ) {
        throw error;
      }
      const apiError = error as {
        response?: { data?: { error?: string } };
        message?: string;
      };
      const errorMessage =
        apiError.response?.data?.error ||
        apiError.message ||
        t("auth.registrationFailed");
      throw new Error(errorMessage);
    }
  };

  const logout = () => {
    localStorage.removeItem("token");
    localStorage.removeItem("user");
    setUser(null);
  };

  const updateUser = (updatedUser: User) => {
    setUser(updatedUser);
    localStorage.setItem("user", JSON.stringify(updatedUser));
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
