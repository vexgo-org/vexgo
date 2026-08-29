import { useState, useEffect } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { isAxiosError } from "axios";
import { postsApi, categoriesApi, uploadApi } from "@/lib/api";
import { useAuth } from "@/hooks/useAuth";
import { useTranslation } from "@/lib/I18nContext";
import type { Category } from "@/types";
import { normalizeTagsArray } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { RichTextEditor } from "@/components/editor/RichTextEditor";
import ImageCropper from "@/components/image/ImageCropper";
import {
  Loader2,
  Save,
  Send,
  X,
  Image as ImageIcon,
  Plus,
  ArrowLeft,
  Trash2,
} from "lucide-react";

// Post fields from the backend may be looser than the frontend Post type (category may be numeric, tags may be an object array)
interface LoadedPost {
  slug?: string;
  title: string;
  content: string;
  excerpt: string;
  coverImage: string | null;
  category?: string | number;
  tags?:
    | string
    | Array<
        | string
        | {
            name?: string;
            Name?: string;
            title?: string;
            label?: string;
            id?: string | number;
          }
      >;
  tag_names?: string[];
  Tags?: Array<{ name?: string; Name?: string; id?: string | number }>;
}

export function WritePostPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const { user, isAuthenticated } = useAuth();
  const isEditMode = !!id;

  // Check if user is authenticated
  useEffect(() => {
    if (!isAuthenticated || !user) {
      alert(t("writePostPage.permissionDenied"));
      navigate("/");
      return;
    }
    // Check if user has permission to create posts
    if (user.role === "guest") {
      alert(t("writePostPage.permissionDenied"));
      navigate("/");
      return;
    }
  }, [isAuthenticated, user, navigate, t]);

  const [slug, setSlug] = useState("");
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [originalContent, setOriginalContent] = useState("");
  const [excerpt, setExcerpt] = useState("");
  const [category, setCategory] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState("");
  const [coverImage, setCoverImage] = useState("");
  const [showCropper, setShowCropper] = useState(false);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [categories, setCategories] = useState<Category[]>([]);
  const [saving, setSaving] = useState(false);
  const [uploadingImage, setUploadingImage] = useState(false);
  const [newCategoryName, setNewCategoryName] = useState("");
  const [creatingCategory, setCreatingCategory] = useState(false);
  const [deletingCategory, setDeletingCategory] = useState(false);

  // Determine the user role
  const isContributor = user?.role === "contributor";

  useEffect(() => {
    const init = async () => {
      await loadCategories();
      if (isEditMode) {
        loadPost();
      }
    };
    init();
    // oxlint-disable-next-line react-hooks/exhaustive-deps -- load once per route id; adding the deps would cause the effect to loop after categories load
  }, [id]);

  const loadCategories = async () => {
    try {
      const response = await categoriesApi.getCategories();
      if (!response || response.data.categories.length === 0) {
        throw Error("no default category");
      }

      // Ensure category ids are strings so they match the Select options (the backend may return numeric ids)
      const normalized = (response.data.categories || []).map((c) => ({
        ...c,
        id: String(c.id),
      }));
      setCategories(normalized);

      if (!isEditMode) {
        const def = normalized.find((c) => c.name.toLowerCase() === "default");
        if (def) {
          setCategory(def.name);
        }
      }
    } catch (error) {
      console.error("Failed to load categories:", error);
    }
  };

  const loadPost = async () => {
    try {
      const response = await postsApi.getPostById(id!);
      const post: LoadedPost = response.data.post;
      console.debug("WritePostPage loaded post:", post);
      console.log("Post content:", post.content);
      console.log("Post content type:", typeof post.content);
      console.log("Post content length:", post.content?.length);
      setSlug(post.slug || "");
      setTitle(post.title);
      setContent(post.content || "");
      setOriginalContent(post.content || "");
      setExcerpt(post.excerpt || "");
      // The backend category may be numeric or a string; resolve it to the matching category name
      if (post.category) {
        const categoryStr = String(post.category);
        // Try to find the matching category in the list
        const foundCategory = categories.find(
          (cat) => String(cat.id) === categoryStr || cat.name === categoryStr,
        );
        // Use the category name as the value when a match is found
        if (foundCategory) {
          setCategory(foundCategory.name);
        } else {
          // Fall back to the raw value when no match is found
          setCategory(categoryStr);
        }
      } else {
        setCategory("");
      }
      // The backend may return tags as an object array [{id,name}, ...]; the frontend needs a string array
      try {
        let mappedTags: string[] = [];
        const rawTags = post.tags;

        if (Array.isArray(rawTags)) {
          if (rawTags.every((x) => typeof x === "string")) {
            // 1) The backend returns a plain string array
            mappedTags = rawTags;
          } else {
            // 2) Common case: an object array [{id,name}, ...]
            mappedTags = rawTags
              .map((t) => {
                if (!t) return "";
                if (typeof t === "string") return t;
                return (
                  t.name ||
                  t.Name ||
                  t.title ||
                  t.label ||
                  (t.id ? String(t.id) : undefined) ||
                  ""
                );
              })
              .filter((v): v is string => Boolean(v));
          }
        } else if (typeof rawTags === "string") {
          // 3) The backend returns a comma-separated string
          mappedTags = rawTags
            .split(",")
            .map((s) => s.trim())
            .filter(Boolean);
        } else if (post.tag_names && Array.isArray(post.tag_names)) {
          mappedTags = post.tag_names
            .map((s) => String(s).trim())
            .filter(Boolean);
        } else if (post.Tags && Array.isArray(post.Tags)) {
          mappedTags = post.Tags.map(
            (t) => t.name || t.Name || (t.id ? String(t.id) : ""),
          ).filter((v): v is string => Boolean(v));
        }

        console.debug(
          "WritePostPage loaded post.tags raw:",
          post.tags,
          "mapped:",
          mappedTags,
        );
        setTags(normalizeTagsArray(mappedTags));
      } catch (e) {
        console.error("Failed to parse post.tags:", e, post && post.tags);
        setTags([]);
      }
      setCoverImage(post.coverImage || "");
    } catch (error) {
      console.error("Failed to load post:", error);
      navigate("/");
    }
  };

  const handleAddTag = () => {
    if (tagInput.trim() && !tags.includes(tagInput.trim())) {
      setTags((prev) => normalizeTagsArray([...prev, tagInput.trim()]));
      setTagInput("");
    }
  };

  const handleRemoveTag = (tagToRemove: string) => {
    setTags(tags.filter((tag) => tag !== tagToRemove));
  };

  const handleCoverImageUpload = async (
    e: React.ChangeEvent<HTMLInputElement>,
  ) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (file.size > 10 * 1024 * 1024) {
      alert(t("writePostPage.imageTooLarge"));
      return;
    }

    // No longer compress the original image: open the cropper so the user can select the cover area
    setSelectedFile(file);
    setShowCropper(true);
  };

  const handleCropConfirm = async (croppedFile: File) => {
    setShowCropper(false);
    setUploadingImage(true);
    try {
      const response = await uploadApi.uploadFile(croppedFile);
      setCoverImage(response.data.file!.url);
    } catch (err) {
      console.error("Failed to upload cropped image:", err);
      alert(t("writePostPage.uploadFailed"));
    } finally {
      setUploadingImage(false);
      setSelectedFile(null);
    }
  };

  // Generate a URL-safe slug from the title
  const generateSlug = () => {
    let s = title
      .normalize("NFKC")
      .toLocaleLowerCase()
      // Convert spaces, underscores, and Unicode dashes to hyphens
      .replace(/[\s_\p{Pd}]+/gu, "-")
      // Keep letters and numbers from all languages
      .replace(/[^\p{L}\p{N}-]/gu, "")
      // Collapse consecutive hyphens and trim leading/trailing hyphens
      .replace(/-+/g, "-")
      .replace(/^-+|-+$/g, "");

    if (!s) {
      s = `post-${Date.now().toString(36)}`;
    }
    setSlug(s.slice(0, 200));
  };

  const handleSubmit = async (status: "published" | "draft" | "pending") => {
    if (!title.trim()) {
      alert(t("writePostPage.titleRequired"));
      return;
    }
    if (!content.trim()) {
      alert(t("writePostPage.contentRequired"));
      return;
    }
    if (!category) {
      alert(t("writePostPage.categoryRequired"));
      return;
    }
    if (!slug.trim()) {
      alert(t("writePostPage.slugRequired"));
      return;
    }

    setSaving(true);
    try {
      const postData = {
        slug: slug.trim(),
        title: title.trim(),
        content,
        category,
        tags,
        excerpt: excerpt.trim(),
        coverImage: coverImage || undefined,
        status: isContributor && status === "published" ? "pending" : status,
      };

      if (isEditMode) {
        const response = await postsApi.updatePost(id!, postData);
        navigate(`/post/${response.data.post.slug}`);
      } else {
        const response = await postsApi.createPost(postData);
        navigate(`/post/${response.data.post.slug}`);
      }
    } catch (error: unknown) {
      console.error("Failed to save post:", error);
      // The backend error body is {"error": string} (409 also carries code: "slug_taken")
      let alertMessage = t("writePostPage.writePostFailed");
      if (isAxiosError<{ error?: string }>(error)) {
        const status = error.response?.status;
        const serverError = error.response?.data?.error;
        if (status === 409) {
          alertMessage = t("writePostPage.slugTaken");
        } else if (status === 400 && serverError) {
          alertMessage = serverError;
        }
      }
      alert(alertMessage);
    } finally {
      setSaving(false);
    }
  };

  const canCreateCategory = isAuthenticated && !!user && user.role !== "guest";
  const handleCreateCategory = async () => {
    const name = newCategoryName.trim();
    if (!name) {
      return;
    }
    setCreatingCategory(true);

    try {
      const res = await categoriesApi.createCategory({ name, description: "" });
      const created = res.data.category;
      const normalized = { ...created, id: String(created.id) };
      setCategories((prev) => [...prev, normalized]);
      setCategory(normalized.name);
      setNewCategoryName("");
    } catch (err) {
      if (isAxiosError<{ error?: string; code?: string }>(err)) {
        if (err.response?.status === 409) {
          alert(t("writePostPage.categoryDuplicate"));
        } else if (err.response?.status === 403) {
          alert(t("writePostPage.categoryCreateForbidden"));
        } else if (err.response?.status === 400 && err.response.data?.error) {
          alert(err.response.data.error);
        } else {
          alert(t("writePostPage.categoryCreateError"));
        }
      } else {
        alert(t("writePostPage.categoryCreateError"));
      }
    } finally {
      setCreatingCategory(false);
    }
  };

  // The category currently chosen in the Select; deletable only when no
  // post references it (the backend enforces this and returns 400 otherwise).
  const selectedCategory = categories.find((cat) => cat.name === category);
  const canDeleteSelected =
    !!selectedCategory && (selectedCategory.postCount ?? 0) === 0;

  const handleDeleteCategory = async () => {
    if (!selectedCategory || !canDeleteSelected) {
      return;
    }
    setDeletingCategory(true);
    try {
      await categoriesApi.deleteCategory(selectedCategory.id);
      setCategories((prev) =>
        prev.filter((cat) => cat.id !== selectedCategory.id),
      );
      setCategory("");
    } catch (err) {
      if (isAxiosError<{ error?: string }>(err)) {
        if (err.response?.status === 403) {
          alert(t("writePostPage.categoryDeleteForbidden"));
        } else if (err.response?.status === 400 && err.response.data?.error) {
          alert(err.response.data.error);
        } else {
          alert(t("writePostPage.categoryDeleteError"));
        }
      } else {
        alert(t("writePostPage.categoryDeleteError"));
      }
    } finally {
      setDeletingCategory(false);
    }
  };

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl">
      {/* Form */}
      <div className="space-y-6">
        {/* Title */}
        <div>
          <Input
            placeholder={t("writePostPage.titlePlaceholder")}
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className="text-2xl font-bold border-0 border-b rounded-none px-0 focus-visible:ring-0"
          />
        </div>

        {/* Slug */}
        <div>
          <Label htmlFor="slug" className="block mb-2">
            {t("writePostPage.slugLabel")} *
          </Label>
          <div className="flex gap-2">
            <Input
              id="slug"
              placeholder={t("writePostPage.slugPlaceholder")}
              value={slug}
              onChange={(e) => setSlug(e.target.value)}
            />
            <Button type="button" variant="outline" onClick={generateSlug}>
              {t("writePostPage.generateSlug")}
            </Button>
          </div>
        </div>

        {/* Cover image */}
        <Card>
          <CardContent className="p-4">
            <Label className="block mb-2">
              {t("writePostPage.coverImage")}
            </Label>
            {coverImage ? (
              <div className="relative">
                <img
                  src={coverImage}
                  alt={t("writePostPage.coverImageAlt")}
                  className="w-full h-48 object-fill rounded-lg"
                />
                <Button
                  variant="destructive"
                  size="sm"
                  className="absolute top-2 right-2"
                  onClick={() => setCoverImage("")}
                >
                  <X className="w-4 h-4" />
                </Button>
              </div>
            ) : (
              <div className="border-2 border-dashed border-gray-300 rounded-lg p-8 text-center">
                <input
                  type="file"
                  accept="image/*"
                  onChange={handleCoverImageUpload}
                  className="hidden"
                  id="cover-image"
                />
                <label
                  htmlFor="cover-image"
                  className="cursor-pointer flex flex-col items-center"
                >
                  {uploadingImage ? (
                    <Loader2 className="w-8 h-8 text-gray-400 animate-spin mb-2" />
                  ) : (
                    <ImageIcon className="w-8 h-8 text-gray-400 mb-2" />
                  )}
                  <span className="text-sm text-gray-500">
                    {uploadingImage
                      ? t("writePostPage.uploading")
                      : t("writePostPage.uploadCover")}
                  </span>
                  <span className="text-xs text-gray-400 mt-1">
                    {t("writePostPage.imageFormat")}
                  </span>
                </label>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Category and tags */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {/* Category */}
          <div>
            <Label htmlFor="category" className="block mb-2">
              {t("writePostPage.categoryLabel")} *
            </Label>
            <Select value={category} onValueChange={setCategory}>
              <SelectTrigger>
                <SelectValue placeholder={t("writePostPage.selectCategory")} />
              </SelectTrigger>
              <SelectContent>
                {categories.map((cat) => (
                  <SelectItem key={cat.id} value={cat.name}>
                    {cat.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {canCreateCategory && (
              <div className="mt-2 flex gap-2">
                <Input
                  className="flex-1"
                  placeholder={t("writePostPage.newCategoryPlaceholder")}
                  value={newCategoryName}
                  onChange={(e) => setNewCategoryName(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      handleCreateCategory();
                    }
                  }}
                />
                <Button
                  type="button"
                  variant="outline"
                  onClick={handleCreateCategory}
                  disabled={creatingCategory}
                >
                  {creatingCategory ? (
                    <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                  ) : (
                    <Plus className="w-4 h-4 mr-2" />
                  )}
                  {t("writePostPage.createCategory")}
                </Button>
                <span
                  title={
                    canDeleteSelected
                      ? t("writePostPage.deleteCategory")
                      : t("writePostPage.categoryInUseHint")
                  }
                >
                  <Button
                    type="button"
                    variant="destructive"
                    onClick={handleDeleteCategory}
                    disabled={deletingCategory || !canDeleteSelected}
                  >
                    {deletingCategory ? (
                      <Loader2 className="w-4 h-4 animate-spin" />
                    ) : (
                      <Trash2 className="w-4 h-4" />
                    )}
                  </Button>
                </span>
              </div>
            )}
          </div>

          {/* Tags */}
          <div>
            <Label htmlFor="tags" className="block mb-2">
              {t("writePostPage.tagsLabel")}
            </Label>
            <div className="flex gap-2">
              <Input
                id="tags"
                placeholder={t("writePostPage.tagsPlaceholder")}
                value={tagInput}
                onChange={(e) => setTagInput(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    handleAddTag();
                  }
                }}
              />
              <Button type="button" onClick={handleAddTag} variant="outline">
                <Plus className="w-4 h-4" />
              </Button>
            </div>
          </div>
        </div>

        {/* Tag display */}
        {tags.length > 0 && (
          <div className="flex flex-wrap gap-2">
            {tags.map((tag, idx) => {
              const display = typeof tag === "string" ? tag : String(tag);
              return (
                <Badge
                  key={`${display}-${idx}`}
                  variant="secondary"
                  className="flex items-center gap-1"
                >
                  {display}
                  <button
                    onClick={() => handleRemoveTag(display)}
                    className="ml-1 hover:text-destructive"
                  >
                    <X className="w-3 h-3" />
                  </button>
                </Badge>
              );
            })}
          </div>
        )}

        {/* Excerpt */}
        <div>
          <Label htmlFor="excerpt" className="block mb-2">
            {t("writePostPage.excerptLabel")}
          </Label>
          <Textarea
            id="excerpt"
            placeholder={t("writePostPage.excerptPlaceholder")}
            value={excerpt}
            onChange={(e) => setExcerpt(e.target.value)}
            rows={3}
          />
        </div>

        {/* Rich text editor */}
        <div>
          <Label className="block mb-2">
            {t("writePostPage.contentLabel")} *
          </Label>
          <RichTextEditor
            content={content}
            onChange={setContent}
            placeholder={t("writePostPage.contentPlaceholder")}
            originalContent={originalContent}
          />
        </div>
        {showCropper && selectedFile && (
          <ImageCropper
            file={selectedFile}
            onCancel={() => {
              setShowCropper(false);
              setSelectedFile(null);
            }}
            onCrop={handleCropConfirm}
          />
        )}
      </div>
      {/* Footer */}
      <div className="flex items-center justify-between mt-4">
        <Button variant="ghost" size="sm" onClick={() => navigate(-1)}>
          <ArrowLeft className="w-4 h-4 mr-2" />
          {t("writePostPage.goBack")}
        </Button>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            onClick={() => handleSubmit("draft")}
            disabled={saving}
          >
            <Save className="w-4 h-4 mr-2" />
            {t("writePostPage.saveDraft")}
          </Button>
          {/* Contributors can only submit posts for review, not publish directly */}
          {isContributor ? (
            <Button onClick={() => handleSubmit("pending")} disabled={saving}>
              {saving ? (
                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
              ) : (
                <Send className="w-4 h-4 mr-2" />
              )}
              {t("writePostPage.submitReview")}
            </Button>
          ) : (
            <Button onClick={() => handleSubmit("published")} disabled={saving}>
              {saving ? (
                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
              ) : (
                <Send className="w-4 h-4 mr-2" />
              )}
              {isEditMode
                ? t("writePostPage.update")
                : t("writePostPage.publish")}
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
