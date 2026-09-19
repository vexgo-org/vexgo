import { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { useNotifications } from "@/hooks/useNotifications";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/PageHeader";
import { getVexGoAPI } from "@/api/generated/endpoints";
import { unwrap } from "@/lib/api";

import { useTranslation } from "@/lib/I18nContext";
import { CreatorApplicationButton } from "@/components/CreatorApplicationButton";

import {
  Bell,
  CheckCircle,
  MessageSquare,
  ThumbsUp,
  Trash2,
  UserPlus,
  Users,
  type LucideIcon,
} from "lucide-react";

// Notification types
type NotificationType = "comment" | "like" | "reply" | "review" | "role";

const TAB_VALUES = [
  "all",
  "unread",
  "comment",
  "like",
  "review",
  "role",
] as const;

type TabValue = (typeof TAB_VALUES)[number];

const TYPE_ICONS: Record<NotificationType, LucideIcon> = {
  comment: MessageSquare,
  reply: MessageSquare,
  like: ThumbsUp,
  review: CheckCircle,
  role: UserPlus,
};

type Notification = {
  id: string;
  type: NotificationType;
  title: string;
  content: string;
  relatedId: string;
  relatedType: "post" | "comment";
  relatedPostId: number | null;
  createdAt: string;
  isRead: boolean;
  sender?: {
    id: string;
    username: string;
    avatar?: string;
  };
};

/**
 * The filter for a tab is a pure function of the tab and the notification, so
 * the whole list is derived once and every tab renders the same component
 * with the visible subset.
 */
function notificationsForTab(
  notifications: Notification[],
  tab: TabValue,
): Notification[] {
  switch (tab) {
    case "all":
      return notifications;
    case "unread":
      return notifications.filter((notification) => !notification.isRead);
    case "comment":
      return notifications.filter(
        (notification) =>
          notification.type === "comment" || notification.type === "reply",
      );
    case "like":
      return notifications.filter(
        (notification) => notification.type === "like",
      );
    case "review":
      return notifications.filter(
        (notification) => notification.type === "review",
      );
    case "role":
      return notifications.filter(
        (notification) => notification.type === "role",
      );
  }
}

export function NotificationCenterPage() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { decrementUnreadCount, clearUnreadCount } = useNotifications();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<TabValue>("all");

  // Notification data
  const [notifications, setNotifications] = useState<Notification[]>([]);

  const [loading, setLoading] = useState(false);

  // Check whether the user is an admin
  const isAdmin = user?.role === "admin" || user?.role === "super_admin";
  // Check whether the user is a guest
  const isGuest = user?.role === "guest";

  // Fetch notifications from the API
  useEffect(() => {
    const fetchNotifications = async () => {
      setLoading(true);
      try {
        const response = await unwrap(getVexGoAPI().getNotifications());
        // Convert the backend data format to the frontend format
        interface RawNotification {
          id: number;
          type: string;
          title: string;
          content: string;
          related_id: string;
          related_type: "post" | "comment";
          related_post_id: number | null;
          created_at: string;
          is_read: boolean;
        }

        const formattedNotifications = (
          response.notifications as RawNotification[]
        ).map((notification) => ({
          id: notification.id.toString(),
          type: notification.type as NotificationType,
          title: notification.title,
          content: notification.content,
          relatedId: notification.related_id,
          relatedType: notification.related_type,
          relatedPostId: notification.related_post_id,
          createdAt: notification.created_at,
          isRead: notification.is_read,
          // The backend may not include sender info, so leave it empty for now
          sender: undefined,
        }));
        setNotifications(formattedNotifications);
      } catch (error) {
        console.error(t("errors.networkError"), error);
      } finally {
        setLoading(false);
      }
    };

    fetchNotifications();
  }, [t]);

  // Mark a notification as read
  const markAsRead = async (id: string) => {
    const wasUnread = notifications.some(
      (notification) => notification.id === id && !notification.isRead,
    );
    try {
      await unwrap(getVexGoAPI().putNotificationsIdRead(Number(id)));
      // Update the local state
      setNotifications((prev) =>
        prev.map((notification) =>
          notification.id === id
            ? { ...notification, isRead: true }
            : notification,
        ),
      );
      if (wasUnread) {
        decrementUnreadCount();
      }
    } catch (error) {
      console.error(t("errors.networkError"), error);
    }
  };

  // Mark all as read
  const markAllAsRead = async () => {
    try {
      await unwrap(getVexGoAPI().putNotificationsReadAll());
      // Update the local state
      setNotifications((prev) =>
        prev.map((notification) => ({ ...notification, isRead: true })),
      );
      clearUnreadCount();
    } catch (error) {
      console.error(t("errors.networkError"), error);
    }
  };

  // Delete a notification
  const deleteNotification = async (id: string) => {
    const wasUnread = notifications.some(
      (notification) => notification.id === id && !notification.isRead,
    );
    try {
      await unwrap(getVexGoAPI().deleteNotificationsId(Number(id)));
      // Update the local state
      setNotifications((prev) =>
        prev.filter((notification) => notification.id !== id),
      );
      if (wasUnread) {
        decrementUnreadCount();
      }
    } catch (error) {
      console.error(t("errors.networkError"), error);
    }
  };

  // Navigate to the related content
  const navigateToRelated = async (
    relatedId: string,
    relatedType: "post" | "comment",
    relatedPostId: number | null,
  ) => {
    // Resolve the post ID: for comment-related notifications use the
    // explicit post ID field; for post notifications relatedId is already
    // the post ID.
    const postId =
      relatedType === "comment" && relatedPostId != null
        ? String(relatedPostId)
        : relatedId;

    if (relatedType === "post") {
      try {
        const response = await unwrap(getVexGoAPI().getPostsByIdId(postId));
        // The post page is served by the public theme, outside this SPA.
        window.location.href = `/post/${response.post?.slug || ""}`;
      } catch {
        // Fallback: the post is managed in the admin console.
        navigate(`/admin/edit-post/${postId}`);
      }
    } else if (relatedType === "comment") {
      // Navigate to the post page and scroll to the comment
      try {
        const response = await unwrap(getVexGoAPI().getPostsByIdId(postId));
        window.location.href = `/post/${response.post?.slug || ""}#comment-${relatedId}`;
      } catch {
        navigate(`/admin/edit-post/${postId}`);
      }
    }
  };

  const openNotification = (notification: Notification) => {
    markAsRead(notification.id);
    if (notification.relatedId) {
      navigateToRelated(
        notification.relatedId,
        notification.relatedType,
        notification.relatedPostId,
      );
    }
  };

  const formatDate = (value: string) => {
    const date = new Date(value);
    return date.toLocaleString(undefined, {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  // Called as a function, not rendered as a component: defining a component
  // inside the parent would give it a new identity every render and remount
  // the whole list.
  const renderNotificationList = (items: Notification[]) => {
    if (items.length === 0) {
      return (
        <Card className="py-0">
          <EmptyState
            icon={Bell}
            title={t("notificationCenter.empty.noNotifications")}
            description={t("notificationCenter.empty.noNotificationsDesc")}
          />
        </Card>
      );
    }

    return (
      <div className="space-y-2">
        {items.map((notification) => {
          const TypeIcon = TYPE_ICONS[notification.type];
          const canOpen = notification.type !== "role";

          return (
            <div
              key={notification.id}
              className="bg-card flex items-stretch rounded-lg border"
            >
              <button
                type="button"
                onClick={() => openNotification(notification)}
                className="hover:bg-muted/40 focus-visible:ring-ring/25 flex min-w-0 flex-1 items-start gap-3 rounded-l-lg p-3 text-left transition-colors outline-none focus-visible:ring-[3px] focus-visible:ring-inset"
              >
                {notification.sender ? (
                  <Avatar className="size-8 shrink-0">
                    <AvatarImage src={notification.sender.avatar} alt="" />
                    <AvatarFallback className="text-[11px]">
                      {notification.sender.username.charAt(0).toUpperCase()}
                    </AvatarFallback>
                  </Avatar>
                ) : (
                  <span
                    className="bg-muted text-muted-foreground mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-md"
                    aria-hidden="true"
                  >
                    <TypeIcon className="size-3.5" />
                  </span>
                )}

                <span className="min-w-0 flex-1">
                  <span className="flex items-baseline justify-between gap-3">
                    <span className="truncate text-[13px] font-medium">
                      {notification.title}
                    </span>
                    <time
                      className="text-muted-foreground shrink-0 text-[11px]"
                      dateTime={notification.createdAt}
                    >
                      {formatDate(notification.createdAt)}
                    </time>
                  </span>
                  <span className="text-muted-foreground mt-0.5 block text-[13px] leading-relaxed">
                    {notification.content}
                  </span>
                  {!notification.isRead && (
                    <Badge variant="outline-info" className="mt-2">
                      {t("notificationCenter.unreadBadge")}
                    </Badge>
                  )}
                </span>
              </button>

              <div className="flex items-center gap-0.5 pr-2">
                {canOpen && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => openNotification(notification)}
                  >
                    {t("notificationCenter.view")}
                  </Button>
                )}
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label={t("common.delete")}
                  className="text-muted-foreground hover:text-destructive"
                  onClick={() => deleteNotification(notification.id)}
                >
                  <Trash2 className="size-4" />
                </Button>
              </div>
            </div>
          );
        })}
      </div>
    );
  };

  const currentTabItems = notificationsForTab(notifications, activeTab);

  return (
    <div className="space-y-6">
      <PageHeader
        title={t("notificationCenter.title")}
        actions={
          <>
            {isGuest && <CreatorApplicationButton />}
            {isAdmin && (
              <Button
                variant="outline"
                onClick={() => navigate("/admin/creator-applications")}
              >
                <Users className="size-4" />
                {t("creatorApplication.reviewApplications")}
              </Button>
            )}
            <Button variant="outline" onClick={markAllAsRead}>
              {t("notificationCenter.markAllAsRead")}
            </Button>
          </>
        }
      />

      <Tabs
        value={activeTab}
        onValueChange={(value) => setActiveTab(value as TabValue)}
        className="w-full"
      >
        <TabsList className="overflow-x-auto">
          {TAB_VALUES.map((tab) => (
            <TabsTrigger key={tab} value={tab}>
              {t(`notificationCenter.tabs.${tab}`)}
            </TabsTrigger>
          ))}
        </TabsList>

        {/* One list, six filters: every tab renders the same rows. */}
        {TAB_VALUES.map((tab) => (
          <TabsContent key={tab} value={tab}>
            {loading ? (
              <div className="space-y-2">
                {[1, 2, 3].map((i) => (
                  <Skeleton key={i} className="h-[86px] rounded-lg" />
                ))}
              </div>
            ) : (
              renderNotificationList(currentTabItems)
            )}
          </TabsContent>
        ))}
      </Tabs>
    </div>
  );
}
