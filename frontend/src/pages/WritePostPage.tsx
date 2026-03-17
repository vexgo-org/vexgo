import { useState, useEffect } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { postsApi, categoriesApi, uploadApi } from "@/lib/api";
import { useAuth } from "@/hooks/useAuth";
import { useTranslation } from "@/lib/I18nContext";
import type { Category } from "@/types";
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
} from "lucide-react";

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

  // 确保 tags 始终为字符串数组，防止把对象直接渲染到 JSX 中导致 React 报错
  const normalizeTagsArray = (raw: any): string[] => {
    if (!raw) return [];
    if (Array.isArray(raw)) {
      return raw
        .map((t: any) => {
          if (!t && t !== 0) return "";
          if (typeof t === "string") return t;
          if (typeof t === "number") return String(t);
          if (typeof t === "object") {
            return (
              t.name ||
              t.Name ||
              t.title ||
              t.label ||
              (t.id ? String(t.id) : "") ||
              ""
            );
          }
          return String(t);
        })
        .map((s: string) => (s ? s.trim() : ""))
        .filter(Boolean);
    }
    if (typeof raw === "string") {
      return raw
        .split(",")
        .map((s: string) => s.trim())
        .filter(Boolean);
    }
    return [];
  };
  // 判断用户角色
  const isContributor = user?.role === "contributor";

  useEffect(() => {
    const init = async () => {
      await loadCategories();
      if (isEditMode) {
        loadPost();
      }
    };
    init();
  }, [id]);

  const loadCategories = async () => {
    try {
      const response = await categoriesApi.getCategories();
      // 确保前端 categories 的 id 为字符串，避免与后端数字 id 类型不一致导致 Select 无法匹配
      const normalized = (response.data.categories || []).map((c: any) => ({
        ...c,
        id: String(c.id),
      }));
      setCategories(normalized);
    } catch (error) {
      console.error("加载分类失败:", error);
    }
  };

  const loadPost = async () => {
    try {
      const response = await postsApi.getPost(id!);
      const post: any = response.data.post;
      console.debug("WritePostPage loaded post:", post);
      console.log("Post content:", post.content);
      console.log("Post content type:", typeof post.content);
      console.log("Post content length:", post.content?.length);
      setTitle(post.title);
      setContent(post.content || "");
      setOriginalContent(post.content || "");
      setExcerpt(post.excerpt || "");
      // 后端的 category 可能是数字或字符串，需要根据分类列表找到对应的分类名称
      if (post.category) {
        const categoryStr = String(post.category);
        // 尝试在分类列表中找到对应的分类
        const foundCategory = categories.find(
          (cat) => String(cat.id) === categoryStr || cat.name === categoryStr,
        );
        // 如果找到对应的分类，使用分类的名称作为 value
        if (foundCategory) {
          setCategory(foundCategory.name);
        } else {
          // 如果没有找到对应的分类，使用原始值
          setCategory(categoryStr);
        }
      } else {
        setCategory("");
      }
      // 后端返回的 tags 为对象数组 [{id,name}, ...]，前端需要字符串数组
      try {
        let mappedTags: string[] = [];

        // 1) 如果后端直接返回字符串数组
        if (
          Array.isArray(post.tags) &&
          post.tags.every((x: any) => typeof x === "string")
        ) {
          mappedTags = post.tags as string[];
        } else if (Array.isArray(post.tags)) {
          // 2) 常见情况：对象数组 [{id,name}, ...]
          mappedTags = post.tags
            .map((t: any) => {
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
            .filter(Boolean);
        } else if (typeof post.tags === "string") {
          // 3) 如果后端返回逗号分隔的字符串
          mappedTags = post.tags
            .split(",")
            .map((s: string) => s.trim())
            .filter(Boolean);
        } else if (post.tag_names && Array.isArray(post.tag_names)) {
          mappedTags = post.tag_names
            .map((s: any) => String(s).trim())
            .filter(Boolean);
        } else if (post.Tags && Array.isArray(post.Tags)) {
          mappedTags = post.Tags.map(
            (t: any) => t.name || t.Name || (t.id ? String(t.id) : ""),
          ).filter(Boolean);
        }

        console.debug(
          "WritePostPage loaded post.tags raw:",
          post.tags,
          "mapped:",
          mappedTags,
        );
        setTags(normalizeTagsArray(mappedTags));
      } catch (e) {
        console.error("解析 post.tags 失败:", e, post && post.tags);
        setTags([]);
      }
      setCoverImage(post.coverImage || "");
    } catch (error) {
      console.error("加载文章失败:", error);
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

    // 不再压缩原图：打开裁剪器，让用户选取封面区域
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
      console.error("上传裁剪后图片失败:", err);
      alert(t("writePostPage.uploadFailed"));
    } finally {
      setUploadingImage(false);
      setSelectedFile(null);
    }
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

    setSaving(true);
    try {
      const postData = {
        title: title.trim(),
        content,
        category,
        tags,
        excerpt: excerpt.trim(),
        coverImage: coverImage || undefined,
        status: isContributor && status === "published" ? "pending" : status,
      };

      if (isEditMode) {
        await postsApi.updatePost(id!, postData);
        navigate(`/post/${id}`);
      } else {
        const response = await postsApi.createPost(postData);
        navigate(`/post/${response.data.post.id}`);
      }
    } catch (error) {
      console.error("保存文章失败:", error);
      alert(t("writePostPage.writePostFailed"));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl">
      {/* 头部 */}
      <div className="flex items-center justify-between mb-8">
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
          {/* 投稿者只能提交待审核文章，不能直接发布 */}
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

      {/* 表单 */}
      <div className="space-y-6">
        {/* 标题 */}
        <div>
          <Input
            placeholder={t("writePostPage.titlePlaceholder")}
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className="text-2xl font-bold border-0 border-b rounded-none px-0 focus-visible:ring-0"
          />
        </div>

        {/* 封面图 */}
        <Card>
          <CardContent className="p-4">
            <Label className="block mb-2">
              {t("writePostPage.coverImage")}
            </Label>
            {coverImage ? (
              <div className="relative">
                <img
                  src={coverImage}
                  alt="封面"
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

        {/* 分类和标签 */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {/* 分类 */}
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
          </div>

          {/* 标签 */}
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

        {/* 标签展示 */}
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

        {/* 摘要 */}
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

        {/* 富文本编辑器 */}
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
    </div>
  );
}