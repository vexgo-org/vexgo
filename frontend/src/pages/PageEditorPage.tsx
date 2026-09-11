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
import { RichTextEditor } from "@/components/editor/RichTextEditor";
import { ArrowLeft, Loader2, Save } from "lucide-react";

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
    <div className="container mx-auto px-4 py-8 max-w-4xl">
      <div className="flex items-center gap-2 mb-6">
        <Button variant="ghost" size="sm" onClick={() => navigate(-1)}>
          <ArrowLeft className="w-4 h-4 mr-1" />
          {t("pageEditorPage.goBack")}
        </Button>
        <h1 className="text-2xl font-bold">
          {isEditMode
            ? t("pageEditorPage.editPage")
            : t("pageEditorPage.newPage")}
        </h1>
      </div>

      <Card>
        <CardContent className="p-6 space-y-4">
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
            <p className="text-xs text-muted-foreground mt-1">
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
            <RichTextEditor
              content={content}
              onChange={setContent}
              placeholder={t("pageEditorPage.contentPlaceholder")}
            />
          </div>
          {error && <p className="text-sm text-destructive">{error}</p>}
          <Button onClick={handleSave} disabled={saving}>
            {saving ? (
              <Loader2 className="w-4 h-4 mr-2 animate-spin" />
            ) : (
              <Save className="w-4 h-4 mr-2" />
            )}
            {t("pageEditorPage.save")}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
