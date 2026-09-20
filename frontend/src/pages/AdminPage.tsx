import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { isAxiosError } from "axios";
import { useAuth } from "@/hooks/useAuth";
import { useTranslation } from "@/lib/I18nContext";
import { getLocale } from "@/lib/i18n";
import { getVexGoAPI } from "@/api/generated/endpoints";
import { unwrap } from "@/lib/api";
import type { Post, Category, Tag as TagType } from "@/types";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton, SkeletonRows } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
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
import { PageHeader } from "@/components/PageHeader";
import { StatusBadge } from "@/components/StatusBadge";

import {
  ChevronRight,
  Cpu,
  Mail,
  MessageSquare,
  Palette,
  PenLine,
  Plus,
  Settings,
  ShieldCheck,
  Trash2,
  Users,
  type LucideIcon,
} from "lucide-react";

/** The admin menu: the same routes as the rail, described for a first visit. */
const ADMIN_LINKS: {
  to: string;
  labelKey: string;
  descriptionKey: string;
  icon: LucideIcon;
}[] = [
  {
    to: "/admin/general-settings",
    labelKey: "generalSettings.title",
    descriptionKey: "adminData.configGeneralSettings",
    icon: Settings,
  },
  {
    to: "/admin/theme",
    labelKey: "themeSettings.title",
    descriptionKey: "adminData.manageThemes",
    icon: Palette,
  },
  {
    to: "/admin/moderation",
    labelKey: "moderation.title",
    descriptionKey: "adminData.manageModeration",
    icon: ShieldCheck,
  },
  {
    to: "/admin/comment-moderation",
    labelKey: "commentModeration.title",
    descriptionKey: "adminData.manageComments",
    icon: MessageSquare,
  },
  {
    to: "/admin/comment-config",
    labelKey: "commentConfig.title",
    descriptionKey: "commentConfig.description",
    icon: MessageSquare,
  },
  {
    to: "/admin/users",
    labelKey: "userManagement.title",
    descriptionKey: "adminData.manageUsers",
    icon: Users,
  },
  {
    to: "/admin/smtp",
    labelKey: "smtpSettings.title",
    descriptionKey: "adminData.configEmail",
    icon: Mail,
  },
  {
    to: "/admin/ai-settings",
    labelKey: "aiSettings.title",
    descriptionKey: "adminData.configAI",
    icon: Cpu,
  },
];

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
      navigate("/admin/my-posts");
      return;
    }

    loadData();
  }, [user, navigate]);

  const loadData = async () => {
    setLoading(true);
    try {
      const [statsRes, postsRes, draftPostsRes, categoriesRes, tagsRes] =
        await Promise.all([
          unwrap(getVexGoAPI().getStats()),
          unwrap(getVexGoAPI().getPosts({ limit: 10 })),
          unwrap(getVexGoAPI().getPostsDrafts({ limit: 10 })),
          unwrap(getVexGoAPI().getCategories()),
          unwrap(getVexGoAPI().getTags()),
        ]);

      setStats({
        posts: statsRes.stats?.posts ?? 0,
        users: statsRes.stats?.users ?? 0,
        comments: statsRes.stats?.comments ?? 0,
        categories: statsRes.stats?.categories ?? 0,
        tags: statsRes.stats?.tags ?? 0,
      });
      setPosts((postsRes.posts as Post[]) || []);
      setDraftPosts((draftPostsRes.posts as Post[]) || []);
      setCategories((categoriesRes.categories as Category[]) || []);
      setTags((tagsRes.tags as TagType[]) || []);
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
      await unwrap(
        getVexGoAPI().postCategories({
          name: newCategoryName,
          description: newCategoryDesc,
        }),
      );
      setNewCategoryName("");
      setNewCategoryDesc("");
      setActionError("");
      loadData();
    } catch (error) {
      console.error("Failed to create category:", error);
      setActionError(apiErrorMessage(error, t("adminData.actionFailed")));
    }
  };

  const handleDeleteCategory = async (categoryId: string | number) => {
    try {
      await unwrap(getVexGoAPI().deleteCategoriesId(Number(categoryId)));
      setActionError("");
      loadData();
    } catch (error) {
      console.error("Failed to delete category:", error);
      setActionError(apiErrorMessage(error, t("adminData.actionFailed")));
    }
  };

  const handleDeleteTag = async (tagId: string | number) => {
    try {
      await unwrap(getVexGoAPI().deleteTagsId(Number(tagId)));
      setActionError("");
      loadData();
    } catch (error) {
      console.error("Failed to delete tag:", error);
      setActionError(apiErrorMessage(error, t("adminData.actionFailed")));
    }
  };

  const handleDeletePost = async (postId: string | number) => {
    try {
      await unwrap(getVexGoAPI().deletePostsId(String(postId)));
      // Stay on the post management page and refresh the data
      setActiveTab("posts");
      loadData();
    } catch (error) {
      console.error("Failed to delete post:", error);
    }
  };

  const handleEditPost = (postId: string) => {
    navigate(`/admin/edit-post/${postId}`);
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
        <PageHeader title={t("admin.title")} />
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
          {[1, 2, 3, 4, 5].map((i) => (
            <div key={i} className="border-t-2 border-rule space-y-2 pt-3">
              <Skeleton className="h-3 w-16" />
              <Skeleton className="h-8 w-12" />
            </div>
          ))}
        </div>
        <SkeletonRows rows={4} />
      </div>
    );
  }

  const statItems = [
    { label: t("admin.totalPosts"), value: stats.posts },
    { label: t("adminData.categories"), value: stats.categories },
    { label: t("adminData.tags"), value: stats.tags },
    { label: t("admin.totalUsers"), value: stats.users },
    { label: t("adminData.comments"), value: stats.comments },
  ];

  /** Shared by the posts and drafts tabs — the only difference is the rows. */
  const renderPostTable = (rows: Post[], emptyLabel: string) =>
    rows.length === 0 ? (
      <EmptyState
        icon={PenLine}
        title={emptyLabel}
        description={t("adminData.noRecords")}
      />
    ) : (
      <div className="bg-card overflow-hidden rounded-md border border-border">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead>{t("posts.title")}</TableHead>
              <TableHead className="w-28">{t("posts.status")}</TableHead>
              <TableHead className="w-32">{t("posts.author")}</TableHead>
              <TableHead className="w-28">{t("common.date")}</TableHead>
              <TableHead className="w-20 text-right">
                <span className="sr-only">{t("posts.edit")}</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((post) => (
              <TableRow key={post.id}>
                <TableCell className="max-w-0">
                  <button
                    type="button"
                    onClick={() => handleEditPost(String(post.id))}
                    className="block max-w-full truncate text-left font-medium hover:underline"
                    title={post.title}
                  >
                    {post.title}
                  </button>
                </TableCell>
                <TableCell>
                  <StatusBadge status={post.status} />
                </TableCell>
                <TableCell className="text-muted-foreground truncate">
                  {post.author?.username ?? "—"}
                </TableCell>
                <TableCell className="text-muted-foreground tabular-nums">
                  {formatDate(post.createdAt || "")}
                </TableCell>
                <TableCell className="text-right">
                  <div className="flex justify-end">
                    <AlertDialog>
                      <AlertDialogTrigger
                        render={
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            aria-label={t("posts.delete")}
                          />
                        }
                      >
                        <Trash2 className="size-4" />
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>
                            {t("posts.deleteConfirm")}
                          </AlertDialogTitle>
                          <AlertDialogDescription>
                            {t("myPostsPage.cannotUndo")}
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>
                            {t("myPostsPage.cancel")}
                          </AlertDialogCancel>
                          <AlertDialogAction
                            onClick={() => handleDeletePost(String(post.id))}
                            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                          >
                            {t("myPostsPage.delete")}
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
    );

  return (
    <div className="space-y-6">
      <PageHeader
        title={t("admin.title")}
        actions={
          <Button render={<Link to="/admin/write" />}>
            <PenLine className="size-4" />
            {t("layout.writePost")}
          </Button>
        }
      />

      {/* Counters, not dashboards: five numbers with no chart to misread. Each
       * one is a figure over a rule with a caption under it, which is how a
       * printed report states a total — no box, no tint. */}
      <dl className="grid grid-cols-2 gap-x-6 gap-y-5 sm:grid-cols-3 lg:grid-cols-5">
        {statItems.map((item) => (
          <div key={item.label} className="border-t-2 border-rule pt-3">
            <dd className="display text-figure tabular-nums">{item.value}</dd>
            <dt className="eyebrow mt-2">{item.label}</dt>
          </div>
        ))}
      </dl>

      <Tabs
        value={activeTab}
        onValueChange={(v) =>
          setActiveTab(
            v as "overview" | "posts" | "drafts" | "categories" | "tags",
          )
        }
        className="w-full"
      >
        <TabsList>
          <TabsTrigger value="overview">{t("adminData.overview")}</TabsTrigger>
          <TabsTrigger value="posts">{t("adminData.posts")}</TabsTrigger>
          <TabsTrigger value="drafts">{t("adminData.draftPosts")}</TabsTrigger>
          <TabsTrigger value="categories">
            {t("adminData.allCategories")}
          </TabsTrigger>
          <TabsTrigger value="tags">{t("adminData.allTags")}</TabsTrigger>
        </TabsList>

        <TabsContent value="overview">
          <ul className="divide-y divide-border border-y border-border">
            {ADMIN_LINKS.map((link) => (
              <li key={link.to}>
                <Link
                  to={link.to}
                  className="hover:bg-accent focus-visible:ring-ring/25 flex items-center gap-3 px-1 py-3 transition-colors outline-none focus-visible:ring-[3px] focus-visible:ring-inset"
                >
                  <link.icon
                    className="text-muted-foreground size-4 shrink-0"
                    aria-hidden="true"
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block text-sm font-medium">
                      {t(link.labelKey)}
                    </span>
                    <span className="text-muted-foreground block truncate text-2xs">
                      {t(link.descriptionKey)}
                    </span>
                  </span>
                  <ChevronRight
                    className="text-muted-foreground/60 size-4 shrink-0"
                    aria-hidden="true"
                  />
                </Link>
              </li>
            ))}
          </ul>
        </TabsContent>

        <TabsContent value="posts">
          {renderPostTable(posts, t("adminData.recentPosts"))}
        </TabsContent>

        <TabsContent value="drafts">
          {renderPostTable(draftPosts, t("adminData.draftPosts"))}
        </TabsContent>

        <TabsContent value="categories" className="space-y-4">
          {actionError && (
            <p className="text-destructive text-sm">{actionError}</p>
          )}
          <Card className="py-4">
            <div className="flex flex-col gap-2 px-5 sm:flex-row">
              <Input
                placeholder={t("adminData.categoryName")}
                aria-label={t("adminData.categoryName")}
                value={newCategoryName}
                onChange={(e) => setNewCategoryName(e.target.value)}
                className="sm:flex-1"
              />
              <Input
                placeholder={t("adminData.categoryDescription")}
                aria-label={t("adminData.categoryDescription")}
                value={newCategoryDesc}
                onChange={(e) => setNewCategoryDesc(e.target.value)}
                className="sm:flex-1"
              />
              <Button onClick={handleCreateCategory}>
                <Plus className="size-4" />
                {t("adminData.add")}
              </Button>
            </div>
          </Card>

          {categories.length === 0 ? (
            <div className="border-t border-border">
              <EmptyState title={t("adminData.noRecords")} />
            </div>
          ) : (
            <div className="bg-card overflow-hidden rounded-md border border-border">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead>{t("adminData.categoryName")}</TableHead>
                    <TableHead>{t("adminData.categoryDescription")}</TableHead>
                    <TableHead className="w-28">
                      {t("adminData.posts")}
                    </TableHead>
                    <TableHead className="w-20 text-right">
                      <span className="sr-only">
                        {t("adminData.deleteCategory")}
                      </span>
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {categories.map((category) => (
                    <TableRow key={category.id}>
                      <TableCell className="font-medium">
                        {category.name}
                      </TableCell>
                      <TableCell className="text-muted-foreground max-w-0">
                        <span className="block truncate">
                          {category.description || t("common.noDescription")}
                        </span>
                      </TableCell>
                      <TableCell className="text-muted-foreground tabular-nums">
                        {category.postCount ?? 0}
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex justify-end">
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            disabled={(category.postCount ?? 0) > 0}
                            title={
                              (category.postCount ?? 0) > 0
                                ? t("adminData.inUseHint")
                                : t("adminData.deleteCategory")
                            }
                            aria-label={t("adminData.deleteCategory")}
                            onClick={() => handleDeleteCategory(category.id)}
                          >
                            <Trash2 className="size-4" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </TabsContent>

        <TabsContent value="tags" className="space-y-4">
          <p className="text-muted-foreground text-xs">
            {t("adminData.tagsHint")}
          </p>
          {tags.length === 0 ? (
            <div className="border-t border-border">
              <EmptyState title={t("adminData.noRecords")} />
            </div>
          ) : (
            <div className="bg-card overflow-hidden rounded-md border border-border">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead>{t("adminData.tags")}</TableHead>
                    <TableHead className="w-28">
                      {t("adminData.posts")}
                    </TableHead>
                    <TableHead className="w-20 text-right">
                      <span className="sr-only">
                        {t("adminData.deleteTag")}
                      </span>
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {tags.map((tag) => (
                    <TableRow key={tag.id}>
                      <TableCell className="font-medium">{tag.name}</TableCell>
                      <TableCell className="text-muted-foreground tabular-nums">
                        {tag.postCount ?? 0}
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex justify-end">
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            disabled={(tag.postCount ?? 0) > 0}
                            title={
                              (tag.postCount ?? 0) > 0
                                ? t("adminData.inUseHint")
                                : t("adminData.deleteTag")
                            }
                            aria-label={t("adminData.deleteTag")}
                            onClick={() => handleDeleteTag(tag.id)}
                          >
                            <Trash2 className="size-4" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </TabsContent>
      </Tabs>
    </div>
  );
}
