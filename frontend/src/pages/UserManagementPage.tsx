import { useCallback, useEffect, useState } from "react";
import { useAuth } from "@/hooks/useAuth";
import { useTranslation } from "@/lib/I18nContext";
import { getVexGoAPI } from "@/api/generated/endpoints";
import { unwrap } from "@/lib/api";
import type { User } from "@/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { SkeletonRows } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { PageHeader } from "@/components/PageHeader";
import { toast } from "sonner";
import { Trash2, Search } from "lucide-react";
import { getLocale } from "@/lib/i18n";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { isUserRole } from "@/lib/utils";

export function UserManagementPage() {
  const { user: currentUser } = useAuth();
  const { t } = useTranslation();
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [currentPage, setCurrentPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [searchTerm, setSearchTerm] = useState("");
  const [searchQuery, setSearchQuery] = useState("");

  // Role display mapping
  const roleDisplayMap: Record<
    string,
    {
      label: string;
      variant: "default" | "secondary" | "destructive" | "outline";
    }
  > = {
    super_admin: { label: t("roles.super_admin"), variant: "destructive" },
    admin: { label: t("roles.admin"), variant: "default" },
    author: { label: t("roles.author"), variant: "secondary" },
    contributor: { label: t("roles.contributor"), variant: "outline" },
    guest: { label: t("roles.guest"), variant: "outline" },
  };

  // Assignable role options (based on the current user's role)
  const getAssignableRoles = () => {
    if (currentUser?.role === "super_admin") {
      return [
        { value: "admin", label: t("roles.admin") },
        { value: "author", label: t("roles.author") },
        { value: "contributor", label: t("roles.contributor") },
        { value: "guest", label: t("roles.guest") },
      ];
    } else if (currentUser?.role === "admin") {
      return [
        { value: "author", label: t("roles.author") },
        { value: "contributor", label: t("roles.contributor") },
        { value: "guest", label: t("roles.guest") },
      ];
    }
    return [];
  };

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const response = await unwrap(
        getVexGoAPI().getUsers({
          page: currentPage,
          limit: 10,
          search: searchQuery,
        }),
      );
      setUsers((response.users || []) as User[]);
      setTotalPages(response.pagination?.totalPages ?? 0);
    } catch (error) {
      console.error("Failed to load user list:", error);
      toast.error(t("userManagement.loadingUsers"));
    } finally {
      setLoading(false);
    }
  }, [currentPage, searchQuery, t]);

  useEffect(() => {
    loadData();
  }, [currentPage, searchQuery, loadData]);

  const handleSearch = () => {
    setSearchQuery(searchTerm);
    setCurrentPage(1); // reset to the first page on search
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      handleSearch();
    }
  };

  const handleRoleChange = async (userId: string, newRole: string) => {
    try {
      if (!isUserRole(newRole)) {
        throw new Error(`Invalid user role: ${newRole}`);
      }

      const response = await unwrap(
        getVexGoAPI().putUsersIdRole(Number(userId), { role: newRole }),
      );
      toast.success(response.message);

      // Update the local user list
      setUsers((prevUsers) =>
        prevUsers.map((user) =>
          user.id === userId
            ? {
                ...user,
                role: newRole as
                  "super_admin" | "admin" | "author" | "contributor" | "guest",
              }
            : user,
        ),
      );
    } catch (error) {
      console.error("Failed to update user role:", error);
      toast.error(t("userManagement.updateRoleFailed"));
    }
  };

  const handleDeleteUser = async (userId: string) => {
    try {
      const response = await unwrap(
        getVexGoAPI().deleteUsersId(Number(userId)),
      );
      toast.success(response.message);

      // The handler is called with `String(user.id)` while `user.id` is a
      // number, so a strict comparison never matched and the deleted row stayed
      // on screen until a reload.
      setUsers((prevUsers) =>
        prevUsers.filter((user) => String(user.id) !== userId),
      );
    } catch (error) {
      console.error("Failed to delete user:", error);
      toast.error(t("userManagement.deleteUserFailed"));
    }
  };

  // Check whether the user can be deleted
  const canDeleteUser = (user: User) => {
    if (currentUser?.id === user.id) return false; // cannot delete yourself
    if (currentUser?.role === "super_admin") return true; // super admins can delete any user
    if (currentUser?.role === "admin") {
      // Admins can only delete authors, contributors, and guests
      return ["author", "contributor", "guest"].includes(user.role);
    }
    return false;
  };

  const formatDate = (dateString: string) => {
    const locale = getLocale();
    return new Date(dateString).toLocaleDateString(locale, {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
  };

  if (loading) {
    return (
      <div className="space-y-6">
        <PageHeader title={t("userManagement.title")} />
        <SkeletonRows rows={5} />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title={t("userManagement.title")}
        actions={
          <div className="flex w-full items-center gap-2 sm:w-auto">
            <div className="relative w-full sm:w-56">
              <Search className="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
              <Input
                type="search"
                placeholder={t("userManagement.searchPlaceholder")}
                aria-label={t("userManagement.searchPlaceholder")}
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                onKeyDown={handleKeyPress}
                className="pl-9"
              />
            </div>
            <Button
              variant="outline"
              size="icon"
              onClick={handleSearch}
              aria-label={t("userManagement.searchPlaceholder")}
            >
              <Search className="size-4" />
            </Button>
          </div>
        }
      />

      <div className="bg-card overflow-hidden rounded-md border border-border">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead>{t("userManagement.userList")}</TableHead>
              <TableHead className="w-28">{t("userManagement.role")}</TableHead>
              <TableHead className="w-32">
                {t("userManagement.registered")}
              </TableHead>
              <TableHead className="w-56 text-right">
                {t("common.actions")}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {users.map((user) => (
              <TableRow key={user.id}>
                <TableCell>
                  <div className="flex items-center gap-2.5">
                    <span className="border-border bg-muted text-muted-foreground flex size-7 shrink-0 items-center justify-center rounded-sm border text-2xs font-medium">
                      {user.username.charAt(0).toUpperCase()}
                    </span>
                    <span className="min-w-0">
                      <span className="block truncate font-medium">
                        {user.username}
                      </span>
                      <span className="text-muted-foreground block truncate text-2xs">
                        {user.email}
                      </span>
                    </span>
                  </div>
                </TableCell>
                <TableCell>
                  <Badge
                    variant={roleDisplayMap[user.role]?.variant || "outline"}
                  >
                    {roleDisplayMap[user.role]?.label || user.role}
                  </Badge>
                </TableCell>
                <TableCell className="text-muted-foreground tabular-nums">
                  {user.createdAt
                    ? formatDate(user.createdAt)
                    : t("userManagement.unknown")}
                </TableCell>
                <TableCell className="text-right">
                  <div className="flex items-center justify-end gap-2">
                    {currentUser?.id === user.id && (
                      <Badge variant="secondary">
                        {t("userManagement.currentUser")}
                      </Badge>
                    )}

                    {currentUser?.id !== user.id &&
                      currentUser?.role === "admin" &&
                      user.role === "admin" && (
                        <Badge variant="secondary">
                          {t("userManagement.sameLevel")}
                        </Badge>
                      )}

                    {currentUser?.id !== user.id &&
                      !(
                        currentUser?.role === "admin" && user.role === "admin"
                      ) && (
                        <Select
                          value={user.role}
                          onValueChange={(value) =>
                            handleRoleChange(String(user.id), value)
                          }
                        >
                          <SelectTrigger className="w-28" size="sm">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {getAssignableRoles().map((role) => (
                              <SelectItem key={role.value} value={role.value}>
                                {role.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      )}

                    {canDeleteUser(user) && (
                      <AlertDialog>
                        <AlertDialogTrigger
                          render={
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              aria-label={t("userManagement.delete")}
                            />
                          }
                        >
                          <Trash2 className="size-4" />
                        </AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>
                              {t("userManagement.deleteConfirmation")}
                            </AlertDialogTitle>
                            <AlertDialogDescription>
                              {t("userManagement.deleteDescription", {
                                username: user.username,
                              })}
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>
                              {t("common.cancel")}
                            </AlertDialogCancel>
                            <AlertDialogAction
                              onClick={() => handleDeleteUser(String(user.id))}
                              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                            >
                              {t("userManagement.delete")}
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {totalPages > 1 && (
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="text-muted-foreground text-xs tabular-nums">
            {t("userManagement.page", { page: currentPage, totalPages })}
          </div>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setCurrentPage(Math.max(1, currentPage - 1))}
              disabled={currentPage === 1}
            >
              {t("userManagement.previousPage")}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() =>
                setCurrentPage(Math.min(totalPages, currentPage + 1))
              }
              disabled={currentPage === totalPages}
            >
              {t("userManagement.nextPage")}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
