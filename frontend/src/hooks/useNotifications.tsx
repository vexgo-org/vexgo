import {
  createContext,
  useContext,
  useState,
  useEffect,
  type ReactNode,
} from "react";
import { useLocation } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { notificationsApi } from "@/lib/api";

interface NotificationContextType {
  unreadCount: number;
  refreshUnreadCount: () => Promise<void>;
  decrementUnreadCount: () => void;
  clearUnreadCount: () => void;
}

const NotificationContext = createContext<NotificationContextType | undefined>(
  undefined,
);

export function NotificationProvider({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth();
  const location = useLocation();
  const [unreadCount, setUnreadCount] = useState(0);

  // Fetch the unread notification count from the backend.
  const refreshUnreadCount = async () => {
    try {
      const response = await notificationsApi.getUnreadCount();
      setUnreadCount(response.data.unreadCount);
    } catch (error) {
      console.error("Failed to fetch the unread notification count:", error);
    }
  };

  // Optimistically reduce the unread count by one, never below zero.
  const decrementUnreadCount = () => {
    setUnreadCount((count) => Math.max(0, count - 1));
  };

  // Optimistically reset the unread count to zero.
  const clearUnreadCount = () => {
    setUnreadCount(0);
  };

  // Fetch the unread count when the user logs in.
  useEffect(() => {
    if (isAuthenticated) {
      refreshUnreadCount();
    }
  }, [isAuthenticated]);

  // Periodically refresh the unread count (every 30s).
  useEffect(() => {
    let interval: ReturnType<typeof setInterval>;
    if (isAuthenticated) {
      interval = setInterval(() => {
        refreshUnreadCount();
      }, 30000); // check every 30 seconds
    }
    return () => {
      if (interval) {
        clearInterval(interval);
      }
    };
  }, [isAuthenticated]);

  // Refresh the unread count on route changes, e.g. when entering or leaving
  // the notifications page.
  useEffect(() => {
    if (isAuthenticated) {
      refreshUnreadCount();
    }
  }, [isAuthenticated, location.pathname]);

  return (
    <NotificationContext.Provider
      value={{
        unreadCount,
        refreshUnreadCount,
        decrementUnreadCount,
        clearUnreadCount,
      }}
    >
      {children}
    </NotificationContext.Provider>
  );
}

export function useNotifications() {
  const context = useContext(NotificationContext);
  if (context === undefined) {
    throw new Error(
      "useNotifications must be used within a NotificationProvider",
    );
  }
  return context;
}
