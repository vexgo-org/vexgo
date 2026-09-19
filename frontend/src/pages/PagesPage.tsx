import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { pagesAPI, type PageItem } from "@/lib/pages";
import { useTranslation } from "@/lib/I18nContext";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { PageHeader } from "@/components/PageHeader";
import { StatusBadge } from "@/components/StatusBadge";
import { Files, Plus, Edit, Trash2, Eye } from "lucide-react";

export function PagesPage() {
  const { t } = useTranslation();
  const [pages, setPages] = useState<PageItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("all");

  const loadPages = useCallback(async () => {
    setLoading(true);
    try {
      const data = await pagesAPI.list({
        page: 1,
        limit: 100,
        status: status === "all" ? "" : status,
        search: search.trim() || undefined,
      });
      setPages(data.pages ?? []);
    } catch (error) {
      console.error("Failed to load pages:", error);
    } finally {
      setLoading(false);
    }
  }, [search, status]);

  useEffect(() => {
    loadPages();
  }, [loadPages]);

  const handleDelete = async (id: number) => {
    try {
      await pagesAPI.remove(id);
      loadPages();
    } catch (error) {
      console.error("Failed to delete page:", error);
    }
  };

  if (loading && pages.length === 0) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-40" />
        <Skeleton className="h-48 rounded-lg" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title={t("pagesPage.title")}
        actions={
          <Button asChild>
            <Link to="/admin/pages/new">
              <Plus className="size-4" />
              {t("pagesPage.newPage")}
            </Link>
          </Button>
        }
      />

      <div className="flex flex-col gap-2 sm:flex-row">
        <Input
          placeholder={t("pagesPage.searchPlaceholder")}
          aria-label={t("pagesPage.searchPlaceholder")}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="sm:max-w-xs"
        />
        <Select value={status} onValueChange={setStatus}>
          <SelectTrigger className="w-full sm:w-36">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("pagesPage.all")}</SelectItem>
            <SelectItem value="published">
              {t("pagesPage.published")}
            </SelectItem>
            <SelectItem value="draft">{t("pagesPage.draft")}</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {pages.length === 0 ? (
        <Card className="py-0">
          <EmptyState
            icon={Files}
            title={t("pagesPage.noPages")}
            action={
              <Button variant="outline" asChild>
                <Link to="/admin/pages/new">
                  <Plus className="size-4" />
                  {t("pagesPage.newPage")}
                </Link>
              </Button>
            }
          />
        </Card>
      ) : (
        <div className="bg-card overflow-hidden rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead>{t("posts.title")}</TableHead>
                <TableHead className="w-48">{t("pagesPage.slug")}</TableHead>
                <TableHead className="w-28">{t("posts.status")}</TableHead>
                <TableHead className="w-32">
                  {t("pagesPage.navigation")}
                </TableHead>
                <TableHead className="w-28 text-right">
                  {t("common.actions")}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {pages.map((page) => (
                <TableRow key={page.id}>
                  <TableCell className="max-w-0">
                    <Link
                      to={`/admin/pages/${page.id}`}
                      className="block truncate font-medium hover:underline"
                      title={page.title}
                    >
                      {page.title}
                    </Link>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    <code className="text-[12px]">/{page.slug}</code>
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={page.status} />
                  </TableCell>
                  <TableCell className="text-muted-foreground text-[12px]">
                    {page.showInNav
                      ? t("pagesPage.inNavWithOrder", {
                          order: page.sortOrder,
                        })
                      : "—"}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-0.5">
                      <Button variant="ghost" size="icon-sm" asChild>
                        <a
                          href={`/${page.slug}`}
                          target="_blank"
                          rel="noreferrer"
                          aria-label={t("common.preview")}
                        >
                          <Eye className="size-4" />
                        </a>
                      </Button>
                      <Button variant="ghost" size="icon-sm" asChild>
                        <Link
                          to={`/admin/pages/${page.id}`}
                          aria-label={t("common.edit")}
                        >
                          <Edit className="size-4" />
                        </Link>
                      </Button>
                      <AlertDialog>
                        <AlertDialogTrigger asChild>
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            aria-label={t("pagesPage.delete")}
                          >
                            <Trash2 className="size-4" />
                          </Button>
                        </AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>
                              {t("pagesPage.confirmDelete")}
                            </AlertDialogTitle>
                            <AlertDialogDescription>
                              {t("pagesPage.cannotUndo")}
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>
                              {t("pagesPage.cancel")}
                            </AlertDialogCancel>
                            <AlertDialogAction
                              onClick={() => handleDelete(page.id)}
                              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                            >
                              {t("pagesPage.delete")}
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
