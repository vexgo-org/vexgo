import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { isAxiosError } from "axios";
import { useAuth } from "@/hooks/useAuth";
import { useTranslation } from "@/lib/I18nContext";
import { getLocale } from "@/lib/i18n";
import { statsApi, postsApi, categoriesApi, tagsApi } from "@/lib/api";
import type { Post, Category, Tag as TagType } from "@/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

import {
  Users,
  FileText,
  MessageSquare,
  Tag,
  Plus,
  Trash2,
  BarChart3,
  Edit,
  Shield,
  CheckCircle,
  XCircle,
  Clock,
  AlertCircle,
  FileX,
  Mail,
  Settings,
  Cpu,
  Palette,
} from "lucide-react";

export function AdminPage() {
  const navigate = useNavigate();
  const { user } = useAuth();
  const { t } = useTranslation();
  const [stats, setStats] = useState({
    posts: 0,
    users: 0,
    comments: 0,
    categories: 0,
    tags: 0,
  });
  const [posts, setPosts] = useState<Post[]>([]);
  const [draftPosts, setDraftPosts] = useState<Post[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [tags, setTags] = useState<TagType[]>([]);
  const [newCategoryName, setNewCategoryName] = useState("");
  const [newCategoryDesc, setNewCategoryDesc] = useState("");
  const [actionError, setActionError] = useState("");
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<
    "overview" | "posts" | "drafts" | "categories" | "tags"
  >("overview");

  useEffect(() => {
    // Check if user is admin or super admin
    if (user && user.role !== "admin" && user.role !== "super_admin") {
      navigate("/");
      return;
    }

    loadData();
  }, [user, navigate]);

  const loadData = async () => {
    setLoading(true);
    try {
      const [statsRes, postsRes, draftPostsRes, categoriesRes, tagsRes] =
        await Promise.all([
          statsApi.getStats(),
          postsApi.getPosts({ limit: 10 }),
          postsApi.getDraftPosts({ limit: 10 }),
          categoriesApi.getCategories(),
          tagsApi.getTags(),
        ]);

      setStats(statsRes.data.stats);
      setPosts(postsRes.data.posts);
      setDraftPosts(draftPostsRes.data.posts);
      setCategories(categoriesRes.data.categories);
      setTags(tagsRes.data.tags);
    } catch (error) {
      console.error("Failed to load data:", error);
    } finally {
      setLoading(false);
    }
  };

  // apiErrorMessage extracts the server-provided error message from a failed
  // API call, falling back to a generic message.
  const apiErrorMessage = (error: unknown, fallback: string) => {
    if (isAxiosError<{ error?: string }>(error) && error.response?.data.error) {
      return error.response.data.error;
    }
    return fallback;
  };

  const handleCreateCategory = async () => {
    if (!newCategoryName.trim()) return;

    try {
      await categoriesApi.createCategory({
        name: newCategoryName,
        description: newCategoryDesc,
      });
      setNewCategoryName("");
      setNewCategoryDesc("");
      setActionError("");
      loadData();
    } catch (error) {
      console.error("Failed to create category:", error);
      setActionError(apiErrorMessage(error, t("adminData.actionFailed")));
    }
  };

  const handleDeleteCategory = async (categoryId: string) => {
    try {
      await categoriesApi.deleteCategory(categoryId);
      setActionError("");
      loadData();
    } catch (error) {
      console.error("Failed to delete category:", error);
      setActionError(apiErrorMessage(error, t("adminData.actionFailed")));
    }
  };

  const handleDeleteTag = async (tagId: string) => {
    try {
      await tagsApi.deleteTag(tagId);
      setActionError("");
      loadData();
    } catch (error) {
      console.error("Failed to delete tag:", error);
      setActionError(apiErrorMessage(error, t("adminData.actionFailed")));
    }
  };

  const handleDeletePost = async (postId: string) => {
    try {
      await postsApi.deletePost(postId);
      // Stay on the post management page and refresh the data
      setActiveTab("posts");
      loadData();
    } catch (error) {
      console.error("Failed to delete post:", error);
    }
  };

  const handleEditPost = (postId: string) => {
    navigate(`/edit-post/${postId}`);
  };

  const formatDate = (dateString: string) => {
    const locale = getLocale();
    return new Date(dateString).toLocaleDateString(locale, {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
  };

  const getStatusBadge = (status: string) => {
    type BadgeVariant = "default" | "secondary" | "destructive" | "outline";
    type IconComponent = typeof CheckCircle;
    const statusMap: Record<
      string,
      {
        label: string;
        variant: BadgeVariant;
        icon: IconComponent;
        className: string;
      }
    > = {
      published: {
        label: t("posts.published"),
        variant: "default",
        icon: CheckCircle,
        className: "bg-green-600 hover:bg-green-700",
      },
      draft: {
        label: t("posts.draft"),
        variant: "secondary",
        icon: FileX,
        className: "",
      },
      pending: {
        label: t("posts.pending"),
        variant: "outline",
        icon: Clock,
        className: "text-yellow-600 border-yellow-600",
      },
      rejected: {
        label: t("posts.rejected"),
        variant: "destructive",
        icon: XCircle,
        className: "",
      },
    };
    return (
      statusMap[status] || {
        variant: "secondary" as BadgeVariant,
        label: status,
        icon: AlertCircle,
        className: "",
      }
    );
  };

  if (loading) {
    return (
      <div className="container mx-auto px-4 py-8">
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
          {[1, 2, 3, 4].map((i) => (
            <Card key={i}>
              <CardContent className="p-6">
                <div className="animate-pulse h-16 bg-muted rounded" />
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="container mx-auto px-4 py-8">
      <div className="flex items-center justify-between mb-8">
        <h1 className="text-2xl font-bold flex items-center gap-2">
          <BarChart3 className="w-6 h-6" />
          {t("admin.title")}
        </h1>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
        <Card>
          <CardContent className="p-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-muted-foreground">
                  {t("admin.totalPosts")}
                </p>
                <p className="text-3xl font-bold">{stats.posts}</p>
              </div>
              <div className="w-12 h-12 bg-blue-100 rounded-lg flex items-center justify-center">
                <FileText className="w-6 h-6 text-blue-600" />
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-muted-foreground">
                  {t("admin.totalUsers")}
                </p>
                <p className="text-3xl font-bold">{stats.users}</p>
              </div>
              <div className="w-12 h-12 bg-green-100 rounded-lg flex items-center justify-center">
                <Users className="w-6 h-6 text-green-600" />
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-muted-foreground">
                  {t("admin.pendingPosts")}
                </p>
                <p className="text-3xl font-bold">{stats.comments}</p>
              </div>
              <div className="w-12 h-12 bg-yellow-100 rounded-lg flex items-center justify-center">
                <MessageSquare className="w-6 h-6 text-yellow-600" />
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-muted-foreground">
                  {t("admin.quickStats")}
                </p>
                <p className="text-3xl font-bold">
                  {stats.categories}/{stats.tags}
                </p>
              </div>
              <div className="w-12 h-12 bg-purple-100 rounded-lg flex items-center justify-center">
                <Tag className="w-6 h-6 text-purple-600" />
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Management Tabs */}
      <Tabs
        value={activeTab}
        onValueChange={(v) =>
          setActiveTab(
            v as "overview" | "posts" | "drafts" | "categories" | "tags",
          )
        }
        className="w-full"
      >
        <TabsList className="grid w-full grid-cols-5 max-w-2xl">
          <TabsTrigger value="overview">{t("adminData.overview")}</TabsTrigger>
          <TabsTrigger value="posts">{t("adminData.posts")}</TabsTrigger>
          <TabsTrigger value="drafts">{t("adminData.draftPosts")}</TabsTrigger>
          <TabsTrigger value="categories">
            {t("adminData.allCategories")}
          </TabsTrigger>
          <TabsTrigger value="tags">{t("adminData.allTags")}</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="mt-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <Card
              className="cursor-pointer hover:shadow-md transition-shadow"
              onClick={() => navigate("/admin/general-settings")}
            >
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Settings className="w-5 h-5" />
                  {t("generalSettings.title")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-muted-foreground">
                  {t("adminData.configGeneralSettings")}
                </p>
              </CardContent>
            </Card>

            <Card
              className="cursor-pointer hover:shadow-md transition-shadow"
              onClick={() => navigate("/admin/moderation")}
            >
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Shield className="w-5 h-5" />
                  {t("moderation.title")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-muted-foreground">
                  {t("adminData.manageModeration")}
                </p>
              </CardContent>
            </Card>

            <Card
              className="cursor-pointer hover:shadow-md transition-shadow"
              onClick={() => navigate("/admin/comment-moderation")}
            >
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <MessageSquare className="w-5 h-5" />
                  {t("commentModeration.title")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-muted-foreground">
                  {t("adminData.manageComments")}
                </p>
              </CardContent>
            </Card>

            <Card
              className="cursor-pointer hover:shadow-md transition-shadow"
              onClick={() => navigate("/admin/users")}
            >
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Users className="w-5 h-5" />
                  {t("userManagement.title")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-muted-foreground">
                  {t("adminData.manageUsers")}
                </p>
              </CardContent>
            </Card>

            <Card
              className="cursor-pointer hover:shadow-md transition-shadow"
              onClick={() => navigate("/admin/smtp")}
            >
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Mail className="w-5 h-5" />
                  {t("smtpSettings.title")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-muted-foreground">
                  {t("adminData.configEmail")}
                </p>
              </CardContent>
            </Card>

            <Card
              className="cursor-pointer hover:shadow-md transition-shadow"
              onClick={() => navigate("/admin/ai-settings")}
            >
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Cpu className="w-5 h-5" />
                  {t("aiSettings.title")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-muted-foreground">
                  {t("adminData.configAI")}
                </p>
              </CardContent>
            </Card>
            <Card
              className="cursor-pointer hover:shadow-md transition-shadow"
              onClick={() => navigate("/admin/comment-config")}
            >
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Settings className="w-5 h-5" />
                  {t("commentConfig.title")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-muted-foreground">
                  {t("commentConfig.description")}
                </p>
              </CardContent>
            </Card>
            <Card
              className="cursor-pointer hover:shadow-md transition-shadow"
              onClick={() => navigate("/admin/theme")}
            >
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Palette className="w-5 h-5" />
                  {t("themeSettings.title")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-muted-foreground">
                  {t("adminData.manageThemes")}
                </p>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="posts" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle>{t("adminData.recentPosts")}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                {posts.map((post) => (
                  <div
                    key={post.id}
                    className="flex items-center justify-between p-4 border rounded-lg"
                  >
                    <div className="flex-1">
                      <div className="flex items-center gap-2 mb-1">
                        {(() => {
                          const status = getStatusBadge(post.status);
                          const IconComponent = status.icon;
                          return (
                            <Badge
                              variant={status.variant}
                              className={status.className}
                            >
                              <IconComponent className="w-3 h-3 mr-1" />
                              {status.label}
                            </Badge>
                          );
                        })()}
                        <span className="text-sm text-muted-foreground">
                          {formatDate(post.createdAt)}
                        </span>
                      </div>
                      <h3 className="font-medium">{post.title}</h3>
                      <p className="text-sm text-muted-foreground">
                        {t("posts.author")}: {post.author?.username}
                      </p>
                    </div>
                    <div className="flex gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => handleEditPost(post.id)}
                      >
                        <Edit className="w-4 h-4" />
                      </Button>
                      <Button
                        variant="destructive"
                        size="sm"
                        onClick={() => handleDeletePost(post.id)}
                      >
                        <Trash2 className="w-4 h-4" />
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="drafts" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle>{t("adminData.draftPosts")}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                {draftPosts.length === 0 ? (
                  <p className="text-center text-muted-foreground py-4">
                    {t("adminData.noRecords")}
                  </p>
                ) : (
                  draftPosts.map((post) => (
                    <div
                      key={post.id}
                      className="flex items-center justify-between p-4 border rounded-lg"
                    >
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-1">
                          <Badge variant="secondary">{t("posts.draft")}</Badge>
                          <span className="text-sm text-muted-foreground">
                            {formatDate(post.createdAt)}
                          </span>
                        </div>
                        <h3 className="font-medium">{post.title}</h3>
                        <p className="text-sm text-muted-foreground">
                          {t("posts.author")}: {post.author?.username}
                        </p>
                      </div>
                      <div className="flex gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => handleEditPost(post.id)}
                        >
                          <Edit className="w-4 h-4" />
                        </Button>
                        <Button
                          variant="destructive"
                          size="sm"
                          onClick={() => handleDeletePost(post.id)}
                        >
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </div>
                    </div>
                  ))
                )}
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="categories" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle>{t("adminData.allCategories")}</CardTitle>
            </CardHeader>
            <CardContent>
              {actionError && (
                <p className="mb-4 text-sm text-destructive">{actionError}</p>
              )}
              {/* Add New Category */}
              <div className="flex gap-4 mb-6">
                <div className="flex-1">
                  <Input
                    placeholder={t("adminData.categoryName")}
                    value={newCategoryName}
                    onChange={(e) => setNewCategoryName(e.target.value)}
                  />
                </div>
                <div className="flex-1">
                  <Input
                    placeholder={t("adminData.categoryDescription")}
                    value={newCategoryDesc}
                    onChange={(e) => setNewCategoryDesc(e.target.value)}
                  />
                </div>
                <Button onClick={handleCreateCategory}>
                  <Plus className="w-4 h-4 mr-2" />
                  {t("adminData.add")}
                </Button>
              </div>

              {/* Category List */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {categories.map((category) => (
                  <div
                    key={category.id}
                    className="p-4 border rounded-lg flex items-start justify-between gap-4"
                  >
                    <div className="flex-1">
                      <h3 className="font-medium">{category.name}</h3>
                      <p className="text-sm text-muted-foreground">
                        {category.description || t("common.noDescription")}
                      </p>
                      <p className="text-sm text-muted-foreground mt-1">
                        {t("adminData.postCount", {
                          count: category.postCount ?? 0,
                        })}
                      </p>
                    </div>
                    <span
                      title={
                        (category.postCount ?? 0) > 0
                          ? t("adminData.inUseHint")
                          : t("adminData.deleteCategory")
                      }
                    >
                      <Button
                        variant="destructive"
                        size="sm"
                        disabled={(category.postCount ?? 0) > 0}
                        onClick={() => handleDeleteCategory(category.id)}
                      >
                        <Trash2 className="w-4 h-4" />
                      </Button>
                    </span>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="tags" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle>{t("adminData.allTags")}</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground mb-4">
                {t("adminData.tagsHint")}
              </p>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {tags.length === 0 ? (
                  <p className="text-center text-muted-foreground py-4">
                    {t("adminData.noRecords")}
                  </p>
                ) : (
                  tags.map((tag) => (
                    <div
                      key={tag.id}
                      className="p-4 border rounded-lg flex items-start justify-between gap-4"
                    >
                      <div>
                        <h3 className="font-medium">{tag.name}</h3>
                        <p className="text-sm text-muted-foreground mt-1">
                          {t("adminData.postCount", {
                            count: tag.postCount ?? 0,
                          })}
                        </p>
                      </div>
                      <span
                        title={
                          (tag.postCount ?? 0) > 0
                            ? t("adminData.inUseHint")
                            : t("adminData.deleteTag")
                        }
                      >
                        <Button
                          variant="destructive"
                          size="sm"
                          disabled={(tag.postCount ?? 0) > 0}
                          onClick={() => handleDeleteTag(tag.id)}
                        >
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </span>
                    </div>
                  ))
                )}
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
