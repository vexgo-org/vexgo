import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { postsApi, categoriesApi, statsApi, likesApi } from '@/lib/api';
import type { Post, Category } from '@/types';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { Skeleton } from '@/components/ui/skeleton';
import { 
  Pagination, 
  PaginationContent, 
  PaginationItem, 
  PaginationLink, 
  PaginationNext, 
  PaginationPrevious 
} from '@/components/ui/pagination';
import { 
  Heart, MessageCircle, Calendar, 
  TrendingUp, Clock, Tag, SearchX 
} from 'lucide-react';

export function HomePage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [posts, setPosts] = useState<Post[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [popularPosts, setPopularPosts] = useState<Post[]>([]);
  const [loading, setLoading] = useState(true);
  const [pagination, setPagination] = useState({
    total: 0,
    page: 1,
    totalPages: 1,
    limit: 10
  });

  const currentPage = parseInt(searchParams.get('page') || '1');
  const searchQuery = searchParams.get('search') || '';
  const selectedCategory = searchParams.get('category') || '';

  useEffect(() => {
    loadCategories();
    loadPopularPosts();
    // 监听来自文章详情页的点赞事件，保持首页与详情页同步
    const handler = (e: any) => {
      try {
        const d = e.detail || {};
        const postId = String(d.postId);
        setPosts((prev) => prev.map((p) => p.id === postId ? { ...p, isLiked: d.isLiked, likesCount: d.likesCount } : p));
        setPopularPosts((prev) => prev.map((p) => p.id === postId ? { ...p, isLiked: d.isLiked, likesCount: d.likesCount } : p));
      } catch (err) {
        // ignore
      }
    };
    window.addEventListener('like-changed', handler as EventListener);

    const commentHandler = (e: any) => {
      try {
        const d = e.detail || {};
        const postId = String(d.postId);
        setPosts((prev) => prev.map((p) => p.id === postId ? { ...p, commentsCount: d.commentsCount } : p));
      } catch (err) {}
    };
    window.addEventListener('comment-changed', commentHandler as EventListener);

    return () => {
      window.removeEventListener('like-changed', handler as EventListener);
      window.removeEventListener('comment-changed', commentHandler as EventListener);
    };
  }, []);

  // 标准化后端返回的文章对象，确保 id/authorId 为字符串，时间为 ISO 字符串
  const normalizePost = (raw: any): Post => {
    if (!raw) return raw;
    return {
      ...raw,
      id: String(raw.id),
      authorId: raw.authorId !== undefined && raw.authorId !== null ? String(raw.authorId) : raw.authorId,
      createdAt: raw.createdAt ? new Date(raw.createdAt).toISOString() : raw.createdAt,
      updatedAt: raw.updatedAt ? new Date(raw.updatedAt).toISOString() : raw.updatedAt,
    } as Post;
  };

  useEffect(() => {
    loadPosts();
  }, [currentPage, searchQuery, selectedCategory]);

  const loadCategories = async () => {
    try {
      const response = await categoriesApi.getCategories();
      setCategories(response.data.categories);
    } catch (error) {
      console.error('加载分类失败:', error);
    }
  };

  const loadPopularPosts = async () => {
    try {
      const response = await statsApi.getPopularPosts(5);
      setPopularPosts(response.data.posts.map((p: any) => normalizePost(p)));
    } catch (error) {
      console.error('加载热门文章失败:', error);
    }
  };

  const loadPosts = async () => {
    setLoading(true);
    try {
      const response = await postsApi.getPosts({
        page: currentPage,
        limit: 10,
        search: searchQuery || undefined,
        category: selectedCategory || undefined
      });
      setPosts(response.data.posts.map((p: any) => normalizePost(p)));
      setPagination(response.data.pagination);
    } catch (error) {
      console.error('加载文章失败:', error);
    } finally {
      setLoading(false);
    }
  };

  const handlePageChange = (page: number) => {
    const newParams = new URLSearchParams(searchParams);
    if (page === 1) {
      newParams.delete('page');
    } else {
      newParams.set('page', page.toString());
    }
    setSearchParams(newParams);
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };

  const handleCategoryClick = (categoryId: string) => {
    const newParams = new URLSearchParams(searchParams);
    if (selectedCategory === categoryId) {
      newParams.delete('category');
    } else {
      newParams.set('category', categoryId);
    }
    newParams.delete('page');
    setSearchParams(newParams);
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: 'short',
      day: 'numeric'
    });
  };

  const handleToggleLike = async (postId: string) => {
    try {
      const response = await likesApi.toggleLike(postId);
      const { isLiked, likesCount } = response.data;
      setPosts((prev) => prev.map((p) => p.id === postId ? { ...p, isLiked, likesCount } : p));
      setPopularPosts((prev) => prev.map((p) => p.id === postId ? { ...p, isLiked, likesCount } : p));
    } catch (error) {
      console.error('切换点赞失败:', error);
    }
  };

  if (loading && posts.length === 0) {
    return (
      <div className="container mx-auto px-4 py-8">
        <div className="grid grid-cols-1 lg:grid-cols-4 gap-8">
          <div className="lg:col-span-3 space-y-6">
            {[1, 2, 3].map((i) => (
              <Card key={i}>
                <CardContent className="p-6">
                  <Skeleton className="h-6 w-3/4 mb-4" />
                  <Skeleton className="h-4 w-full mb-2" />
                  <Skeleton className="h-4 w-2/3" />
                </CardContent>
              </Card>
            ))}
          </div>
          <div className="space-y-6">
            <Skeleton className="h-48" />
            <Skeleton className="h-48" />
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="container mx-auto px-4 py-8">
      {/* 搜索提示 */}
      {searchQuery && (
        <div className="mb-6 flex items-center gap-2">
          <SearchX className="w-5 h-5 text-muted-foreground" />
          <span className="text-muted-foreground">
            搜索 &quot;{searchQuery}&quot; 的结果
          </span>
          <Button 
            variant="ghost" 
            size="sm" 
            onClick={() => {
              const newParams = new URLSearchParams(searchParams);
              newParams.delete('search');
              setSearchParams(newParams);
            }}
          >
            清除搜索
          </Button>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-4 gap-8">
        {/* 主内容区 */}
        <div className="lg:col-span-3 space-y-6">
          {posts.length === 0 ? (
            <Card>
              <CardContent className="p-12 text-center">
                <div className="w-16 h-16 bg-muted rounded-full flex items-center justify-center mx-auto mb-4">
                  <SearchX className="w-8 h-8 text-muted-foreground" />
                </div>
                <h3 className="text-lg font-semibold mb-2">没有找到文章</h3>
                <p className="text-muted-foreground">
                  {searchQuery ? '尝试使用其他关键词搜索' : '暂无文章，快来发布第一篇吧！'}
                </p>
              </CardContent>
            </Card>
          ) : (
            <>
              <div className="space-y-6">
                {posts.map((post) => (
                  <Card key={post.id} className="group hover:shadow-lg transition-shadow">
                    <CardContent className="p-6">
                      {/* 封面图 */}
                      {post.coverImage && (
                        <Link to={`/post/${post.id}`}>
                          <div className="mb-4 overflow-hidden rounded-lg">
                            <img
                              src={post.coverImage}
                              alt={post.title}
                              className="w-full h-48 object-cover group-hover:scale-105 transition-transform duration-300"
                            />
                          </div>
                        </Link>
                      )}

                      {/* 分类和标签 */}
                      <div className="flex flex-wrap items-center gap-2 mb-3">
                        {post.categoryInfo && (
                          <Badge variant="secondary">
                            {post.categoryInfo.name}
                          </Badge>
                        )}
                        {post.tags?.slice(0, 3).map((tag) => (
                          <Badge key={tag} variant="outline" className="text-xs">
                            {tag}
                          </Badge>
                        ))}
                      </div>

                      {/* 标题 */}
                      <Link to={`/post/${post.id}`}>
                        <h2 className="text-xl font-bold mb-3 group-hover:text-primary transition-colors line-clamp-2">
                          {post.title}
                        </h2>
                      </Link>

                      {/* 摘要 */}
                      <p className="text-muted-foreground mb-4 line-clamp-2">
                        {post.excerpt}
                      </p>

                      {/* 作者和统计 */}
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-3">
                          <Avatar className="w-8 h-8">
                            <AvatarFallback className="bg-primary/10 text-primary text-sm">
                              {post.author?.username?.charAt(0).toUpperCase()}
                            </AvatarFallback>
                          </Avatar>
                          <div className="flex items-center gap-2 text-sm text-muted-foreground">
                            <span>{post.author?.username}</span>
                            <span>·</span>
                            <span className="flex items-center gap-1">
                              <Calendar className="w-3 h-3" />
                              {formatDate(post.createdAt)}
                            </span>
                          </div>
                        </div>

                        <div className="flex items-center gap-4 text-sm">
                          <button
                            onClick={() => handleToggleLike(post.id)}
                            className={`flex items-center gap-1 ${post.isLiked ? 'text-red-500' : 'text-muted-foreground'}`}
                          >
                            <Heart className={`w-4 h-4 ${post.isLiked ? 'fill-current' : ''}`} />
                            <span>{post.likesCount || 0}</span>
                          </button>
                          <span className="flex items-center gap-1 text-muted-foreground">
                            <MessageCircle className="w-4 h-4" />
                            {post.commentsCount || 0}
                          </span>
                        </div>
                      </div>
                    </CardContent>
                  </Card>
                ))}
              </div>

              {/* 分页 */}
              {pagination.totalPages > 1 && (
                <Pagination>
                  <PaginationContent>
                    <PaginationItem>
                      <PaginationPrevious
                        onClick={() => handlePageChange(currentPage - 1)}
                        className={currentPage === 1 ? 'pointer-events-none opacity-50' : 'cursor-pointer'}
                      />
                    </PaginationItem>

                    {Array.from({ length: pagination.totalPages }, (_, i) => i + 1)
                      .filter(page => 
                        page === 1 || 
                        page === pagination.totalPages || 
                        Math.abs(page - currentPage) <= 1
                      )
                      .map((page, index, array) => (
                        <div key={page} className="flex items-center">
                          {index > 0 && array[index - 1] !== page - 1 && (
                            <span className="px-2 text-muted-foreground">...</span>
                          )}
                          <PaginationItem>
                            <PaginationLink
                              isActive={page === currentPage}
                              onClick={() => handlePageChange(page)}
                            >
                              {page}
                            </PaginationLink>
                          </PaginationItem>
                        </div>
                      ))}

                    <PaginationItem>
                      <PaginationNext
                        onClick={() => handlePageChange(currentPage + 1)}
                        className={currentPage === pagination.totalPages ? 'pointer-events-none opacity-50' : 'cursor-pointer'}
                      />
                    </PaginationItem>
                  </PaginationContent>
                </Pagination>
              )}
            </>
          )}
        </div>

        {/* 侧边栏 */}
        <div className="space-y-6">
          {/* 分类 */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg flex items-center gap-2">
                <Tag className="w-4 h-4" />
                分类
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex flex-wrap gap-2">
                {categories.map((category) => (
                  <Badge
                    key={category.id}
                    variant={selectedCategory === category.id ? 'default' : 'secondary'}
                    className="cursor-pointer"
                    onClick={() => handleCategoryClick(category.id)}
                  >
                    {category.name}
                  </Badge>
                ))}
              </div>
            </CardContent>
          </Card>

          {/* 热门文章 */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg flex items-center gap-2">
                <TrendingUp className="w-4 h-4" />
                热门文章
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                {popularPosts.map((post, index) => (
                  <Link 
                    key={post.id} 
                    to={`/post/${post.id}`}
                    className="flex items-start gap-3 group"
                  >
                    <span className="text-lg font-bold text-muted-foreground w-6">
                      {index + 1}
                    </span>
                    <div>
                      <h4 className="text-sm font-medium line-clamp-2 group-hover:text-primary transition-colors">
                        {post.title}
                      </h4>
                      <div className="flex items-center gap-2 mt-1 text-xs text-muted-foreground">
                        <Heart className="w-3 h-3" />
                        {post.likesCount || 0}
                      </div>
                    </div>
                  </Link>
                ))}
              </div>
            </CardContent>
          </Card>

          {/* 关于 */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg flex items-center gap-2">
                <Clock className="w-4 h-4" />
                关于 BlogHub
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">
                BlogHub 是一个现代化的博客平台，支持富文本编辑、图片视频上传、评论互动等功能。
                快来分享你的故事吧！
              </p>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
