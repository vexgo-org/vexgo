import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { AuthProvider } from '@/hooks/useAuth';
import { Layout } from '@/components/Layout';
import { ProtectedRoute } from '@/components/ProtectedRoute';
import { HomePage } from '@/pages/HomePage';
import { PostDetailPage } from '@/pages/PostDetailPage';
import { WritePostPage } from '@/pages/WritePostPage';
import { LoginPage } from '@/pages/LoginPage';
import { RegisterPage } from '@/pages/RegisterPage';
import { ResetPasswordPage } from '@/pages/ResetPasswordPage';
import { ProfilePage } from '@/pages/ProfilePage';
import { MyPostsPage } from '@/pages/MyPostsPage';
import { AdminPage } from '@/pages/AdminPage';
import { SettingsPage } from '@/pages/SettingsPage';
import { ModerationPage } from '@/pages/ModerationPage';
import { UserManagementPage } from '@/pages/UserManagementPage';
import { SMTPSettingsPage } from '@/pages/SMTPSettingsPage';
import { GeneralSettingsPage } from '@/pages/GeneralSettingsPage';
import { VerifyEmailPage } from '@/pages/VerifyEmailPage';
import { CommentModerationPage } from '@/pages/CommentModerationPage';
import { CommentConfigPage } from '@/pages/CommentConfigPage';
import { AISettingsPage } from '@/pages/AISettingsPage';
import { Toaster } from '@/components/ui/sonner';

function App() {
  return (
    <AuthProvider>
      <Router>
        <Layout>
          <Routes>
            {/* 公开路由 */}
            <Route path="/" element={<HomePage />} />
            <Route path="/post/:id" element={<PostDetailPage />} />
            <Route path="/login" element={<LoginPage />} />
            <Route path="/register" element={<RegisterPage />} />
            <Route path="/reset-password" element={<ResetPasswordPage />} />
            <Route path="/verify-email" element={<VerifyEmailPage />} />

            {/* 需要登录的路由 */}
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
              path="/settings"
              element={
                <ProtectedRoute>
                  <SettingsPage />
                </ProtectedRoute>
              }
            />

            {/* 需要管理员权限的路由 */}
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
          </Routes>
        </Layout>
      </Router>
      <Toaster />
    </AuthProvider>
  );
}

export default App;