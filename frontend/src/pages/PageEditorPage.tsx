import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { isAxiosError } from "axios";
import {
  pagesAPI,
  normalizePageSlug,
  isValidPageSlug,
  RESERVED_PAGE_SLUGS,
} from "@/lib/pages";
import { useTranslation } from "@/lib/I18nContext";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { MarkdownEditor } from "@/components/editor";
import { ArrowLeft, Save } from "lucide-react";
import { Spinner } from "@/components/ui/spinner";

export function PageEditorPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const isEditMode = !!id && id !== "new";

  const [title, setTitle] = useState("");
  const [slug, setSlug] = useState("");
  const [content, setContent] = useState("");
  const [showInNav, setShowInNav] = useState(false);
  const [sortOrder, setSortOrder] = useState(0);
  const [status, setStatus] = useState("draft");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!isEditMode) return;
    let cancelled = false;
    (async () => {
      try {
        // Load via admin list then match by id (no public by-id endpoint).
        const data = await pagesAPI.list({ page: 1, limit: 100 });
        const found = (data.pages ?? []).find(
          (p) => String(p.id) === String(id),
        );
        if (!found || cancelled) return;
        setTitle(found.title);
        setSlug(found.slug);
        setContent(found.content);
        setShowInNav(found.showInNav);
        setSortOrder(found.sortOrder);
        setStatus(found.status);
      } catch (e) {
        console.error("Failed to load page:", e);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [id, isEditMode]);

  const generateSlug = () => {
    const base = title
      .toLowerCase()
      .replace(/[^a-z0-9\s-]/g, "")
      .trim()
      .replace(/[\s_]+/g, "-")
      .replace(/-+/g, "-");
    if (base) setSlug(base);
  };

  const handleSave = async () => {
    setError("");
    const normalized = normalizePageSlug(slug);
    if (!title.trim()) {
      setError(t("pageEditorPage.titleRequired"));
      return;
    }
    if (!normalized) {
      setError(t("pageEditorPage.slugRequired"));
      return;
    }
    if (RESERVED_PAGE_SLUGS.has(normalized)) {
      setError(t("pageEditorPage.slugReserved"));
      return;
    }
    if (!isValidPageSlug(normalized)) {
      setError(t("pageEditorPage.slugInvalid"));
      return;
    }
    if (!content.trim()) {
      setError(t("pageEditorPage.contentRequired"));
      return;
    }
    setSaving(true);
    try {
      if (isEditMode) {
        await pagesAPI.update(id as string, {
          slug: normalized,
          title: title.trim(),
          content,
          showInNav,
          sortOrder: Number(sortOrder) || 0,
          status,
        });
      } else {
        await pagesAPI.create({
          slug: normalized,
          title: title.trim(),
          content,
          showInNav,
          sortOrder: Number(sortOrder) || 0,
          status,
        });
      }
      navigate("/admin/pages");
    } catch (e) {
      if (isAxiosError(e) && e.response?.status === 409) {
        setError(t("pageEditorPage.slugTaken"));
      } else if (isAxiosError(e) && e.response?.status === 400) {
        const msg =
          (e.response.data as { error?: string })?.error ??
          t("pageEditorPage.saveFailed");
        setError(msg);
      } else {
        setError(t("pageEditorPage.saveFailed"));
      }
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div className="flex items-center gap-2">
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={() => navigate(-1)}
          aria-label={t("pageEditorPage.goBack")}
        >
          <ArrowLeft className="size-4" />
        </Button>
        <h1 className="display text-title">
          {isEditMode
            ? t("pageEditorPage.editPage")
            : t("pageEditorPage.newPage")}
        </h1>
      </div>

      <Card>
        <CardContent className="space-y-4">
          <div>
            <Label htmlFor="page-title">{t("pageEditorPage.title")}</Label>
            <Input
              id="page-title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder={t("pageEditorPage.titlePlaceholder")}
            />
          </div>
          <div>
            <Label htmlFor="page-slug">{t("pageEditorPage.slug")}</Label>
            <div className="flex gap-2">
              <Input
                id="page-slug"
                value={slug}
                onChange={(e) => setSlug(e.target.value)}
                placeholder="about"
              />
              <Button type="button" variant="outline" onClick={generateSlug}>
                {t("pageEditorPage.generateSlug")}
              </Button>
            </div>
            <p className="text-muted-foreground mt-1.5 font-mono text-2xs">
              /{normalizePageSlug(slug) || "slug"}
            </p>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <div>
              <Label>{t("pageEditorPage.status")}</Label>
              <Select value={status} onValueChange={setStatus}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="draft">
                    {t("pageEditorPage.draft")}
                  </SelectItem>
                  <SelectItem value="published">
                    {t("pageEditorPage.published")}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label htmlFor="page-sort">{t("pageEditorPage.sortOrder")}</Label>
              <Input
                id="page-sort"
                type="number"
                value={sortOrder}
                onChange={(e) => setSortOrder(Number(e.target.value))}
              />
            </div>
            <div className="flex items-end gap-2 pb-2">
              <Checkbox
                id="page-nav"
                checked={showInNav}
                onCheckedChange={(v) => setShowInNav(v === true)}
              />
              <Label htmlFor="page-nav">{t("pageEditorPage.showInNav")}</Label>
            </div>
          </div>
          <div>
            <Label>{t("pageEditorPage.content")}</Label>
            <MarkdownEditor
              value={content}
              onChange={setContent}
              placeholder={t("pageEditorPage.contentPlaceholder")}
            />
          </div>
          {error && <p className="text-sm text-destructive">{error}</p>}
          <Button onClick={handleSave} disabled={saving}>
            {saving ? (
              <Spinner className="size-4" />
            ) : (
              <Save className="size-4" />
            )}
            {t("pageEditorPage.save")}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
