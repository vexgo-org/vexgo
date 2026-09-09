import type { GeneratedAPI } from "./auth.js";

export function createNotifications(api: GeneratedAPI) {
  return {
    list: (
      params?: Parameters<GeneratedAPI["getNotifications"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["getNotifications"]>>> =>
      api.getNotifications(params),
    unreadCount: (): Promise<
      Awaited<ReturnType<GeneratedAPI["getNotificationsUnreadCount"]>>
    > => api.getNotificationsUnreadCount(),
    markRead: (
      id: Parameters<GeneratedAPI["putNotificationsIdRead"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["putNotificationsIdRead"]>>> =>
      api.putNotificationsIdRead(id),
    markAllRead: (): Promise<
      Awaited<ReturnType<GeneratedAPI["putNotificationsReadAll"]>>
    > => api.putNotificationsReadAll(),
    remove: (
      id: Parameters<GeneratedAPI["deleteNotificationsId"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["deleteNotificationsId"]>>> =>
      api.deleteNotificationsId(id),
  };
}

export type NotificationsClient = ReturnType<typeof createNotifications>;
