import { BrowserRouter as Router, Routes, Route } from "react-router-dom";
import { AuthProvider } from "@/hooks/useAuth";
import { NotificationProvider } from "@/hooks/useNotifications";
import { I18nProvider } from "@/lib/I18nContext";
import { Layout } from "@/components/Layout";
import { ProtectedRoute } from "@/components/ProtectedRoute";
import { HomePage } from "@/pages/HomePage";
import { PostDetailPage } from "@/pages/PostDetailPage";
import { WritePostPage } from "@/pages/WritePostPage";
import { LoginPage } from "@/pages/LoginPage";
import { RegisterPage } from "@/pages/RegisterPage";
import { ResetPasswordPage } from "@/pages/ResetPasswordPage";
import { ProfilePage } from "@/pages/ProfilePage";
import { MyPostsPage } from "@/pages/MyPostsPage";
import { UserPostsPage } from "@/pages/UserPostsPage";
import { AdminPage } from "@/pages/AdminPage";
import { ThemePage } from "@/pages/ThemePage";
import { SettingsPage } from "@/pages/SettingsPage";
import { ModerationPage } from "@/pages/ModerationPage";
import { UserManagementPage } from "@/pages/UserManagementPage";
import { SMTPSettingsPage } from "@/pages/SMTPSettingsPage";
import { GeneralSettingsPage } from "@/pages/GeneralSettingsPage";
import { VerifyEmailPage } from "@/pages/VerifyEmailPage";
import { CommentModerationPage } from "@/pages/CommentModerationPage";
import { CommentConfigPage } from "@/pages/CommentConfigPage";
import { AISettingsPage } from "@/pages/AISettingsPage";
import { NotificationCenterPage } from "@/pages/NotificationCenterPage";
import { CreatorApplicationReviewPage } from "@/pages/CreatorApplicationReviewPage";
import { NotFoundPage } from "@/pages/NotFoundPage";
import { Toaster } from "@/components/ui/sonner";
import { useEffect } from "react";

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
              <Routes>
                {/* Public routes */}
                <Route path="/" element={<HomePage />} />
                <Route path="/post/:slug" element={<PostDetailPage />} />
                <Route path="/user/:id" element={<UserPostsPage />} />
                <Route path="/login" element={<LoginPage />} />
                <Route path="/register" element={<RegisterPage />} />
                <Route path="/reset-password" element={<ResetPasswordPage />} />
                <Route path="/verify-email" element={<VerifyEmailPage />} />

                {/* Routes requiring login */}
                <Route
                  path="/write"
                  element={
                    <ProtectedRoute>
                      <WritePostPage />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/edit-post/:id"
                  element={
                    <ProtectedRoute>
                      <WritePostPage />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/profile"
                  element={
                    <ProtectedRoute>
                      <ProfilePage />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/my-posts"
                  element={
                    <ProtectedRoute>
                      <MyPostsPage />
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/notifications"
                  element={
                    <ProtectedRoute>
                      <NotificationCenterPage />
                    </ProtectedRoute>
                  }
                />

                <Route
                  path="/settings"
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
                {/* Catch-all 404 */}
                <Route path="*" element={<NotFoundPage />} />
              </Routes>
            </Layout>
          </NotificationProvider>
        </Router>
        <Toaster />
      </I18nProvider>
    </AuthProvider>
  );
}

export default App;
