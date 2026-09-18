import { BrowserRouter as Router, Routes, Route } from "react-router-dom";
import { lazy, Suspense, useEffect } from "react";
import { AuthProvider } from "@/hooks/useAuth";
import { NotificationProvider } from "@/hooks/useNotifications";
import { I18nProvider } from "@/lib/I18nContext";
import { Layout } from "@/components/Layout";
import { ProtectedRoute } from "@/components/ProtectedRoute";
import { Spinner } from "@/components/ui/spinner";
import { Toaster } from "@/components/ui/sonner";

// Pages are code-split per route: the shell only ships the layout, the auth
// and i18n providers, and the API client, while a route's payload — most of
// all the editor stack behind /admin/write and /admin/pages/:id — is fetched
// when that route is first visited.
const WritePostPage = lazy(() =>
  import("@/pages/WritePostPage").then((m) => ({ default: m.WritePostPage })),
);
const LoginPage = lazy(() =>
  import("@/pages/LoginPage").then((m) => ({ default: m.LoginPage })),
);
const RegisterPage = lazy(() =>
  import("@/pages/RegisterPage").then((m) => ({ default: m.RegisterPage })),
);
const ResetPasswordPage = lazy(() =>
  import("@/pages/ResetPasswordPage").then((m) => ({
    default: m.ResetPasswordPage,
  })),
);
const ProfilePage = lazy(() =>
  import("@/pages/ProfilePage").then((m) => ({ default: m.ProfilePage })),
);
const MyPostsPage = lazy(() =>
  import("@/pages/MyPostsPage").then((m) => ({ default: m.MyPostsPage })),
);
const AdminPage = lazy(() =>
  import("@/pages/AdminPage").then((m) => ({ default: m.AdminPage })),
);
const ThemePage = lazy(() =>
  import("@/pages/ThemePage").then((m) => ({ default: m.ThemePage })),
);
const SettingsPage = lazy(() =>
  import("@/pages/SettingsPage").then((m) => ({ default: m.SettingsPage })),
);
const ModerationPage = lazy(() =>
  import("@/pages/ModerationPage").then((m) => ({ default: m.ModerationPage })),
);
const UserManagementPage = lazy(() =>
  import("@/pages/UserManagementPage").then((m) => ({
    default: m.UserManagementPage,
  })),
);
const SMTPSettingsPage = lazy(() =>
  import("@/pages/SMTPSettingsPage").then((m) => ({
    default: m.SMTPSettingsPage,
  })),
);
const GeneralSettingsPage = lazy(() =>
  import("@/pages/GeneralSettingsPage").then((m) => ({
    default: m.GeneralSettingsPage,
  })),
);
const VerifyEmailPage = lazy(() =>
  import("@/pages/VerifyEmailPage").then((m) => ({
    default: m.VerifyEmailPage,
  })),
);
const CommentModerationPage = lazy(() =>
  import("@/pages/CommentModerationPage").then((m) => ({
    default: m.CommentModerationPage,
  })),
);
const CommentConfigPage = lazy(() =>
  import("@/pages/CommentConfigPage").then((m) => ({
    default: m.CommentConfigPage,
  })),
);
const AISettingsPage = lazy(() =>
  import("@/pages/AISettingsPage").then((m) => ({
    default: m.AISettingsPage,
  })),
);
const NotificationCenterPage = lazy(() =>
  import("@/pages/NotificationCenterPage").then((m) => ({
    default: m.NotificationCenterPage,
  })),
);
const CreatorApplicationReviewPage = lazy(() =>
  import("@/pages/CreatorApplicationReviewPage").then((m) => ({
    default: m.CreatorApplicationReviewPage,
  })),
);
const PagesPage = lazy(() =>
  import("@/pages/PagesPage").then((m) => ({ default: m.PagesPage })),
);
const PageEditorPage = lazy(() =>
  import("@/pages/PageEditorPage").then((m) => ({
    default: m.PageEditorPage,
  })),
);
const NotFoundPage = lazy(() =>
  import("@/pages/NotFoundPage").then((m) => ({ default: m.NotFoundPage })),
);

function RouteFallback() {
  return (
    <div className="min-h-[50vh] flex items-center justify-center">
      <Spinner className="size-8 text-primary" />
    </div>
  );
}

function App() {
  // Apply theme on app startup
  useEffect(() => {
    const savedTheme = localStorage.getItem("theme") as
      "light" | "dark" | "system" | null;
    const theme = savedTheme || "light";

    document.documentElement.classList.remove("light", "dark");
    if (theme === "dark") {
      document.documentElement.classList.add("dark");
    } else if (theme === "light") {
      document.documentElement.classList.add("light");
    } else {
      // Follow the system to remove specific theme classes
      const prefersDark = window.matchMedia(
        "(prefers-color-scheme: dark)",
      ).matches;
      if (prefersDark) {
        document.documentElement.classList.add("dark");
      } else {
        document.documentElement.classList.add("light");
      }
    }
  }, []);

  return (
    <AuthProvider>
      <I18nProvider>
        <Router>
          <NotificationProvider>
            <Layout>
              <Suspense fallback={<RouteFallback />}>
                <Routes>
                  {/* Auth routes */}
                  <Route path="/admin/login" element={<LoginPage />} />
                  <Route path="/admin/register" element={<RegisterPage />} />
                  <Route
                    path="/admin/reset-password"
                    element={<ResetPasswordPage />}
                  />
                  <Route
                    path="/admin/verify-email"
                    element={<VerifyEmailPage />}
                  />

                  {/* Routes requiring login */}
                  <Route
                    path="/admin/write"
                    element={
                      <ProtectedRoute>
                        <WritePostPage />
                      </ProtectedRoute>
                    }
                  />
                  <Route
                    path="/admin/edit-post/:id"
                    element={
                      <ProtectedRoute>
                        <WritePostPage />
                      </ProtectedRoute>
                    }
                  />
                  <Route
                    path="/admin/profile"
                    element={
                      <ProtectedRoute>
                        <ProfilePage />
                      </ProtectedRoute>
                    }
                  />
                  <Route
                    path="/admin/my-posts"
                    element={
                      <ProtectedRoute>
                        <MyPostsPage />
                      </ProtectedRoute>
                    }
                  />
                  <Route
                    path="/admin/notifications"
                    element={
                      <ProtectedRoute>
                        <NotificationCenterPage />
                      </ProtectedRoute>
                    }
                  />

                  <Route
                    path="/admin/settings"
                    element={
                      <ProtectedRoute>
                        <SettingsPage />
                      </ProtectedRoute>
                    }
                  />

                  {/* Routes requiring admin access */}
                  <Route
                    path="/admin"
                    element={
                      <ProtectedRoute requireAdmin>
                        <AdminPage />
                      </ProtectedRoute>
                    }
                  />
                  <Route
                    path="/admin/moderation"
                    element={
                      <ProtectedRoute requireAdmin>
                        <ModerationPage />
                      </ProtectedRoute>
                    }
                  />
                  <Route
                    path="/admin/users"
                    element={
                      <ProtectedRoute requireAdmin>
                        <UserManagementPage />
                      </ProtectedRoute>
                    }
                  />
                  <Route
                    path="/admin/smtp"
                    element={
                      <ProtectedRoute requireAdmin>
                        <SMTPSettingsPage />
                      </ProtectedRoute>
                    }
                  />
                  <Route
                    path="/admin/general-settings"
                    element={
                      <ProtectedRoute requireAdmin>
                        <GeneralSettingsPage />
                      </ProtectedRoute>
                    }
                  />
                  <Route
                    path="/admin/comment-moderation"
                    element={
                      <ProtectedRoute requireAdmin>
                        <CommentModerationPage />
                      </ProtectedRoute>
                    }
                  />
                  <Route
                    path="/admin/comment-config"
                    element={
                      <ProtectedRoute requireAdmin>
                        <CommentConfigPage />
                      </ProtectedRoute>
                    }
                  />
                  <Route
                    path="/admin/ai-settings"
                    element={
                      <ProtectedRoute requireAdmin>
                        <AISettingsPage />
                      </ProtectedRoute>
                    }
                  />
                  <Route
                    path="/admin/theme"
                    element={
                      <ProtectedRoute requireAdmin>
                        <ThemePage />
                      </ProtectedRoute>
                    }
                  />
                  <Route
                    path="/admin/creator-applications"
                    element={
                      <ProtectedRoute requireAdmin>
                        <CreatorApplicationReviewPage />
                      </ProtectedRoute>
                    }
                  />
                  <Route
                    path="/admin/pages"
                    element={
                      <ProtectedRoute requireAdmin>
                        <PagesPage />
                      </ProtectedRoute>
                    }
                  />
                  <Route
                    path="/admin/pages/new"
                    element={
                      <ProtectedRoute requireAdmin>
                        <PageEditorPage />
                      </ProtectedRoute>
                    }
                  />
                  <Route
                    path="/admin/pages/:id"
                    element={
                      <ProtectedRoute requireAdmin>
                        <PageEditorPage />
                      </ProtectedRoute>
                    }
                  />
                  {/* Catch-all 404 */}
                  <Route path="*" element={<NotFoundPage />} />
                </Routes>
              </Suspense>
            </Layout>
          </NotificationProvider>
        </Router>
        <Toaster />
      </I18nProvider>
    </AuthProvider>
  );
}

export default App;
