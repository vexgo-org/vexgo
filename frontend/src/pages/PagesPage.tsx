import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { pagesAPI, type PageItem } from "@/lib/pages";
import { useTranslation } from "@/lib/I18nContext";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
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
import { FileText, Plus, Edit, Trash2, Eye, Clock } from "lucide-react";

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
      <div className="container mx-auto px-4 py-8 max-w-4xl">
        <Skeleton className="h-8 w-40 mb-6" />
        {[1, 2].map((i) => (
          <Card key={i} className="mb-4">
            <CardContent className="p-6">
              <Skeleton className="h-6 w-3/4 mb-4" />
              <Skeleton className="h-4 w-1/2" />
            </CardContent>
          </Card>
        ))}
      </div>
    );
  }

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold flex items-center gap-2">
          <FileText className="w-6 h-6" />
          {t("pagesPage.title")}
        </h1>
        <Button asChild>
          <Link to="/admin/pages/new">
            <Plus className="w-4 h-4 mr-2" />
            {t("pagesPage.newPage")}
          </Link>
        </Button>
      </div>

      <div className="flex gap-2 mb-4">
        <Input
          placeholder={t("pagesPage.searchPlaceholder")}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-xs"
        />
        <Select value={status} onValueChange={setStatus}>
          <SelectTrigger className="w-36">
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
        <Card>
          <CardContent className="p-12 text-center">
            <p className="text-muted-foreground mb-4">
              {t("pagesPage.noPages")}
            </p>
            <Button asChild>
              <Link to="/admin/pages/new">
                <Plus className="w-4 h-4 mr-2" />
                {t("pagesPage.newPage")}
              </Link>
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {pages.map((page) => (
            <Card key={page.id}>
              <CardContent className="p-6">
                <div className="flex items-start justify-between gap-4">
                  <div className="flex-1">
                    <div className="flex items-center gap-2 mb-2">
                      <Badge
                        variant={
                          page.status === "published" ? "default" : "secondary"
                        }
                      >
                        {page.status === "published"
                          ? t("pagesPage.published")
                          : t("pagesPage.draft")}
                      </Badge>
                      {page.showInNav && (
                        <Badge variant="outline">{t("pagesPage.inNav")}</Badge>
                      )}
                      <span className="text-sm text-muted-foreground flex items-center gap-1">
                        <Clock className="w-3 h-3" />/{page.slug} · #
                        {page.sortOrder}
                      </span>
                    </div>
                    <h2 className="text-lg font-semibold mb-1">{page.title}</h2>
                  </div>
                  <div className="flex flex-col gap-2">
                    <Button variant="outline" size="sm" asChild>
                      <a
                        href={`/${page.slug}`}
                        target="_blank"
                        rel="noreferrer"
                      >
                        <Eye className="w-4 h-4" />
                      </a>
                    </Button>
                    <Button variant="outline" size="sm" asChild>
                      <Link to={`/admin/pages/${page.id}`}>
                        <Edit className="w-4 h-4" />
                      </Link>
                    </Button>
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button variant="destructive" size="sm">
                          <Trash2 className="w-4 h-4" />
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
                            className="bg-destructive"
                          >
                            {t("pagesPage.delete")}
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
