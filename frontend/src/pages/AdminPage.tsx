import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import { statsApi, postsApi, categoriesApi } from '@/lib/api';
import type { Post, Category } from '@/types';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';

import {
  Users, FileText, MessageSquare, Tag,
  Plus, Trash2, BarChart3, Edit, Shield
} from 'lucide-react';

export function AdminPage() {
  const navigate = useNavigate();
  const { user } = useAuth();
  const [stats, setStats] = useState({
    posts: 0,
    users: 0,
    comments: 0,
    categories: 0,
    tags: 0
  });
  const [posts, setPosts] = useState<Post[]>([]);
  const [draftPosts, setDraftPosts] = useState<Post[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [newCategoryName, setNewCategoryName] = useState('');
  const [newCategoryDesc, setNewCategoryDesc] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // 检查是否为管理员或超级管理员
    if (user && user.role !== 'admin' && user.role !== 'super_admin') {
      navigate('/');
      return;
    }

    loadData();
  }, [user]);

  const loadData = async () => {
    setLoading(true);
    try {
      const [statsRes, postsRes, draftPostsRes, categoriesRes] = await Promise.all([
        statsApi.getStats(),
        postsApi.getPosts({ limit: 10 }),
        postsApi.getDraftPosts({ limit: 10 }),
        categoriesApi.getCategories()
      ]);

      setStats(statsRes.data.stats);
      setPosts(postsRes.data.posts);
      setDraftPosts(draftPostsRes.data.posts);
      setCategories(categoriesRes.data.categories);
    } catch (error) {
      console.error('加载数据失败:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreateCategory = async () => {
    if (!newCategoryName.trim()) return;

    try {
      await categoriesApi.createCategory({
        name: newCategoryName,
        description: newCategoryDesc
      });
      setNewCategoryName('');
      setNewCategoryDesc('');
      loadData();
    } catch (error) {
      console.error('创建分类失败:', error);
    }
  };

  const handleDeletePost = async (postId: string) => {
    try {
      await postsApi.deletePost(postId);
      loadData();
    } catch (error) {
      console.error('删除文章失败:', error);
    }
  };

  const handleEditPost = (postId: string) => {
    navigate(`/edit-post/${postId}`);
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
          管理后台
        </h1>
      </div>

      {/* 统计卡片 */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
        <Card>
          <CardContent className="p-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-muted-foreground">文章总数</p>
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
                <p className="text-sm text-muted-foreground">用户总数</p>
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
                <p className="text-sm text-muted-foreground">评论总数</p>
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
                <p className="text-sm text-muted-foreground">分类/标签</p>
                <p className="text-3xl font-bold">{stats.categories}/{stats.tags}</p>
              </div>
              <div className="w-12 h-12 bg-purple-100 rounded-lg flex items-center justify-center">
                <Tag className="w-6 h-6 text-purple-600" />
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* 管理标签页 */}
      <Tabs defaultValue="overview" className="w-full">
        <TabsList className="grid w-full grid-cols-4 max-w-md">
          <TabsTrigger value="overview">概览</TabsTrigger>
          <TabsTrigger value="posts">文章管理</TabsTrigger>
          <TabsTrigger value="drafts">草稿管理</TabsTrigger>
          <TabsTrigger value="categories">分类管理</TabsTrigger>
        </TabsList>
        
        <TabsContent value="overview" className="mt-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <Card className="cursor-pointer hover:shadow-md transition-shadow" onClick={() => navigate('/admin/moderation')}>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Shield className="w-5 h-5" />
                  内容审核
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-muted-foreground">管理待审核的文章内容</p>
              </CardContent>
            </Card>
            
            <Card className="cursor-pointer hover:shadow-md transition-shadow" onClick={() => navigate('/admin/users')}>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Users className="w-5 h-5" />
                  用户管理
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-muted-foreground">管理系统用户和权限</p>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="posts" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle>最近文章</CardTitle>
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
                        <Badge variant={post.status === 'published' ? 'default' : 'secondary'}>
                          {post.status === 'published' ? '已发布' : '草稿'}
                        </Badge>
                        <span className="text-sm text-muted-foreground">
                          {formatDate(post.createdAt)}
                        </span>
                      </div>
                      <h3 className="font-medium">{post.title}</h3>
                      <p className="text-sm text-muted-foreground">
                        作者: {post.author?.username}
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
              <CardTitle>草稿文章</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                {draftPosts.length === 0 ? (
                  <p className="text-center text-muted-foreground py-4">暂无草稿文章</p>
                ) : (
                  draftPosts.map((post) => (
                    <div 
                      key={post.id} 
                      className="flex items-center justify-between p-4 border rounded-lg"
                    >
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-1">
                          <Badge variant="secondary">草稿</Badge>
                          <span className="text-sm text-muted-foreground">
                            {formatDate(post.createdAt)}
                          </span>
                        </div>
                        <h3 className="font-medium">{post.title}</h3>
                        <p className="text-sm text-muted-foreground">
                          作者: {post.author?.username}
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
              <CardTitle>分类管理</CardTitle>
            </CardHeader>
            <CardContent>
              {/* 添加新分类 */}
              <div className="flex gap-4 mb-6">
                <div className="flex-1">
                  <Input
                    placeholder="分类名称"
                    value={newCategoryName}
                    onChange={(e) => setNewCategoryName(e.target.value)}
                  />
                </div>
                <div className="flex-1">
                  <Input
                    placeholder="分类描述（可选）"
                    value={newCategoryDesc}
                    onChange={(e) => setNewCategoryDesc(e.target.value)}
                  />
                </div>
                <Button onClick={handleCreateCategory}>
                  <Plus className="w-4 h-4 mr-2" />
                  添加
                </Button>
              </div>

              {/* 分类列表 */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {categories.map((category) => (
                  <div 
                    key={category.id} 
                    className="p-4 border rounded-lg"
                  >
                    <h3 className="font-medium">{category.name}</h3>
                    <p className="text-sm text-muted-foreground">
                      {category.description || '暂无描述'}
                    </p>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
