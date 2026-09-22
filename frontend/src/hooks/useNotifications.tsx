import {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  type ReactNode,
} from "react";
import { useLocation } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { getVexGoAPI } from "@/api/generated/endpoints";

const POLL_INTERVAL_MS = 30_000;

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
  const refreshUnreadCount = useCallback(async () => {
    try {
      const response = await getVexGoAPI().getNotificationsUnreadCount();
      setUnreadCount(response.data.unreadCount ?? 0);
    } catch (error) {
      console.error("Failed to fetch the unread notification count:", error);
    }
  }, []);

  // Optimistically reduce the unread count by one, never below zero.
  const decrementUnreadCount = useCallback(() => {
    setUnreadCount((count) => Math.max(0, count - 1));
  }, []);

  // Optimistically reset the unread count to zero.
  const clearUnreadCount = useCallback(() => {
    setUnreadCount(0);
  }, []);

  /* Poll on an interval, but only while the tab is visible: a hidden tab shows
     no badge, so its requests are pure waste and would keep running for as long
     as the tab stayed open in the background. Coming back to the tab refreshes
     once, since the count may have moved on while nothing was polling. */
  useEffect(() => {
    if (!isAuthenticated) return;

    let interval: ReturnType<typeof setInterval> | undefined;

    const stop = () => {
      if (interval === undefined) return;
      clearInterval(interval);
      interval = undefined;
    };

    const start = () => {
      if (interval !== undefined) return;
      interval = setInterval(() => {
        void refreshUnreadCount();
      }, POLL_INTERVAL_MS);
    };

    const onVisibilityChange = () => {
      if (document.hidden) {
        stop();
        return;
      }
      void refreshUnreadCount();
      start();
    };

    if (!document.hidden) {
      start();
    }
    document.addEventListener("visibilitychange", onVisibilityChange);

    return () => {
      document.removeEventListener("visibilitychange", onVisibilityChange);
      stop();
    };
  }, [isAuthenticated, refreshUnreadCount]);

  /* Refresh on navigation — entering or leaving the notifications page changes
     what the badge should show. This is also the only initial fetch; a separate
     mount effect used to fire alongside it and send the same request twice. */
  useEffect(() => {
    if (isAuthenticated) {
      void refreshUnreadCount();
    }
  }, [isAuthenticated, location.pathname, refreshUnreadCount]);

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
