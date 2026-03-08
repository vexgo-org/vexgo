import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Card, CardContent } from '@/components/ui/card';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Badge } from '@/components/ui/badge';
import { Separator } from '@/components/ui/separator';
import { ArrowLeft, Calendar, MessageSquare, Heart } from 'lucide-react';
import { postsApi } from '@/lib/api';
import type { Post, User } from '@/types';

export function UserPostsPage() {
  const { id } = useParams<{ id: string }>();
  const [posts, setPosts] = useState<Post[]>([]);
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [currentPage, setCurrentPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);

  useEffect(() => {
    if (id) {
      loadUserPosts();
    }
  }, [id, currentPage]);

  const loadUserPosts = async () => {
    setLoading(true);
    try {
      const response = await postsApi.getUserPosts(id!, { page: currentPage, limit: 10 });
      setPosts(response.data.posts);
      setTotalPages(response.data.pagination.totalPages);
      // 从第一篇文章中获取用户信息
      if (response.data.posts.length > 0 && response.data.posts[0].author) {
        setUser(response.data.posts[0].author);
      }
    } catch (error) {
      console.error('加载用户文章失败:', error);
    } finally {
      setLoading(false);
    }
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: 'short',
      day: 'numeric'
    });
  };

  if (loading) {
    return (
      <div className="container mx-auto px-4 py-8 max-w-6xl">
        <Button variant="ghost" size="sm" asChild className="mb-6">
          <Link to="/" className="flex items-center gap-2">
            <ArrowLeft className="w-4 h-4" />
            返回首页
          </Link>
        </Button>

        <div className="flex flex-col md:flex-row gap-6">
          {/* 用户信息 */}
          <div className="w-full md:w-64">
            <Card>
              <CardContent className="p-6">
                <div className="flex flex-col items-center text-center">
                  <Skeleton className="w-24 h-24 rounded-full mb-4" />
                  <Skeleton className="h-6 w-32 mb-2" />
                  <Skeleton className="h-4 w-24 mb-4" />
                  <Skeleton className="h-4 w-48" />
                </div>
              </CardContent>
            </Card>
          </div>

          {/* 文章列表 */}
          <div className="flex-1">
            {[1, 2, 3].map((i) => (
              <Card key={i} className="mb-4">
                <CardContent className="p-6">
                  <Skeleton className="h-6 w-3/4 mb-4" />
                  <Skeleton className="h-4 w-1/2 mb-4" />
                  <div className="flex items-center justify-between">
                    <Skeleton className="h-4 w-32" />
                    <Skeleton className="h-4 w-24" />
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="container mx-auto px-4 py-8 max-w-6xl">
      {/* 返回按钮 */}
      <Button variant="ghost" size="sm" asChild className="mb-6">
        <Link to="/" className="flex items-center gap-2">
          <ArrowLeft className="w-4 h-4" />
          返回首页
        </Link>
      </Button>

      <div className="flex flex-col md:flex-row gap-6">
        {/* 用户信息 */}
        <div className="w-full md:w-64">
          <Card className="sticky top-8">
            <CardContent className="p-6">
              <div className="flex flex-col items-center text-center">
                <Avatar className="w-24 h-24 mb-4">
                  {user?.avatar ? (
                    <img src={user.avatar} alt="Avatar" className="w-full h-full object-cover" />
                  ) : (
                    <AvatarFallback className="bg-primary/10 text-primary text-2xl">
                      {user?.username?.charAt(0).toUpperCase() || 'U'}
                    </AvatarFallback>
                  )}
                </Avatar>
                <h2 className="text-xl font-bold mb-2">{user?.username || '未知用户'}</h2>
                {!JSON.parse(localStorage.getItem('userSettings') || '{}').hideEmail && (
                  <p className="text-sm text-muted-foreground mb-2">{user?.email || ''}</p>
                )}
                
                {/* 生日和个性签名 */}
                <div className="flex flex-col gap-1 mb-4 text-sm text-muted-foreground">
                  {user?.birthday && !JSON.parse(localStorage.getItem('userSettings') || '{}').hideBirthday && (
                    <div className="flex items-center gap-1">
                      <Calendar className="w-3 h-3" />
                      <span>生日: {new Date(user.birthday).toLocaleDateString('zh-CN')}</span>
                    </div>
                  )}
                  {user?.bio && !JSON.parse(localStorage.getItem('userSettings') || '{}').hideBio && (
                    <p className="text-sm text-muted-foreground">{user.bio}</p>
                  )}
                </div>
                
                <Separator className="my-4" />
                <div className="w-full">
                  <p className="text-sm text-muted-foreground mb-2">文章总数</p>
                  <p className="text-2xl font-bold">{posts.length > 0 ? posts.length : 0}</p>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* 文章列表 */}
        <div className="flex-1">
          <h1 className="text-2xl font-bold mb-6">{user?.username || '未知用户'}的文章</h1>
          
          {posts.length === 0 ? (
            <Card>
              <CardContent className="p-12 text-center">
                <h3 className="text-lg font-semibold mb-2">还没有文章</h3>
                <p className="text-muted-foreground">该用户还没有发表任何文章</p>
              </CardContent>
            </Card>
          ) : (
            <div className="space-y-4">
              {posts.map((post) => (
                <Card key={post.id} className="hover:shadow-md transition-shadow">
                  <CardContent className="p-6">
                    <Link to={`/post/${post.id}`} className="block group">
                      <h2 className="text-xl font-bold mb-3 group-hover:text-primary transition-colors">
                        {post.title}
                      </h2>
                      <p className="text-muted-foreground mb-4 line-clamp-2">
                        {post.excerpt || post.content.substring(0, 100) + '...'}
                      </p>
                    </Link>
                    
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <Avatar className="w-8 h-8">
                          {post.author?.avatar ? (
                            <img src={post.author.avatar} alt="Avatar" className="w-full h-full object-cover" />
                          ) : (
                            <AvatarFallback className="bg-primary/10 text-primary text-sm">
                              {post.author?.username?.charAt(0).toUpperCase() || 'U'}
                            </AvatarFallback>
                          )}
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
                      
                      <div className="flex items-center gap-4 text-sm text-muted-foreground">
                        <Badge variant="outline">{post.category}</Badge>
                        <span className="flex items-center gap-1">
                          <MessageSquare className="w-3 h-3" />
                          {post.commentsCount || 0}
                        </span>
                        <span className="flex items-center gap-1">
                          <Heart className="w-3 h-3" />
                          {post.likesCount || 0}
                        </span>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}

          {/* 分页 */}
          {totalPages > 1 && (
            <div className="flex items-center justify-between mt-6">
              <div className="text-sm text-muted-foreground">
                第 {currentPage} 页，共 {totalPages} 页
              </div>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setCurrentPage(Math.max(1, currentPage - 1))}
                  disabled={currentPage === 1}
                >
                  上一页
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setCurrentPage(Math.min(totalPages, currentPage + 1))}
                  disabled={currentPage === totalPages}
                >
                  下一页
                </Button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}