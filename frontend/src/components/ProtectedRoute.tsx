import { Navigate, useLocation } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { Spinner } from "@/components/ui/spinner";

interface ProtectedRouteProps {
  children: React.ReactNode;
  requireAdmin?: boolean;
}

export function ProtectedRoute({
  children,
  requireAdmin = false,
}: ProtectedRouteProps) {
  const { isAuthenticated, user, isLoading } = useAuth();
  const location = useLocation();

  if (isLoading) {
    return (
      <div className="bg-background flex min-h-screen items-center justify-center">
        <Spinner className="text-muted-foreground size-5" />
      </div>
    );
  }

  if (!isAuthenticated) {
    return <Navigate to="/admin/login" state={{ from: location }} replace />;
  }

  // Check whether admin access is required (admin or super_admin)
  if (requireAdmin && user?.role !== "admin" && user?.role !== "super_admin") {
    return <Navigate to="/admin/my-posts" replace />;
  }

  return <>{children}</>;
}
