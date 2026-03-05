import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { getPendingPosts, getApprovedPosts, getRejectedPosts, approvePost, rejectPost, resubmitPost } from '@/lib/moderationApi';
import type { Post } from '@/types';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  CheckCircle, XCircle, Clock, AlertCircle,
  Eye, Edit, Send
} from 'lucide-react';
import { toast } from 'sonner';

export function ModerationPage() {
  const navigate = useNavigate();
  const [pendingPosts, setPendingPosts] = useState<Post[]>([]);
  const [approvedPosts, setApprovedPosts] = useState<Post[]>([]);
  const [rejectedPosts, setRejectedPosts] = useState<Post[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState('pending');

  useEffect(() => {
    loadData();
  }, [activeTab]);

  const loadData = async () => {
    setLoading(true);
    try {
      if (activeTab === 'pending') {
        const response = await getPendingPosts({ limit: 100 });
        setPendingPosts(response.data.posts);
      } else if (activeTab === 'approved') {
        const response = await getApprovedPosts({ limit: 100 });
        setApprovedPosts(response.data.posts);
      } else if (activeTab === 'rejected') {
        const response = await getRejectedPosts({ limit: 100 });
        setRejectedPosts(response.data.posts);
      }
    } catch (error) {
      console.error('加载数据失败:', error);
      toast.error('加载数据失败');
    } finally {
      setLoading(false);
    }
  };

  const handleApprovePost = async (postId: string) => {
    try {
      await approvePost(postId);
      toast.success('文章审核通过');
      loadData();
    } catch (error) {
      console.error('审核通过失败:', error);
      toast.error('审核通过失败');
    }
  };

  const handleRejectPost = async (postId: string) => {
    try {
      await rejectPost(postId);
      toast.success('文章已拒绝');
      loadData();
    } catch (error) {
      console.error('拒绝文章失败:', error);
      toast.error('拒绝文章失败');
    }
  };

  const handleResubmitPost = async (postId: string) => {
    try {
      await resubmitPost(postId);
      toast.success('文章已重新提交审核');
      loadData();
    } catch (error) {
      console.error('重新提交审核失败:', error);
      toast.error('重新提交审核失败');
    }
  };

  const handleViewPost = (postId: string) => {
    navigate(`/post/${postId}`);
  };

  const handleEditPost = (postId: string) => {
    navigate(`/edit-post/${postId}`);
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  };

  if (loading) {
    return (
      <div className="container mx-auto px-4 py-8">
        <div className="flex items-center justify-between mb-8">
          <h1 className="text-2xl font-bold">内容审核</h1>
        </div>
        <div className="space-y-4">
          {[1, 2, 3].map((i) => (
            <Card key={i}>
              <CardContent className="p-6">
                <div className="animate-pulse space-y-4">
                  <div className="h-6 bg-muted rounded w-3/4"></div>
                  <div className="h-4 bg-muted rounded w-1/2"></div>
                  <div className="h-4 bg-muted rounded w-1/4"></div>
                </div>
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
        <h1 className="text-2xl font-bold">内容审核</h1>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
        <TabsList className="grid w-full grid-cols-3 max-w-md">
          <TabsTrigger value="pending" className="flex items-center gap-2">
            <Clock className="w-4 h-4" />
            待审核 ({pendingPosts.length})
          </TabsTrigger>
          <TabsTrigger value="approved" className="flex items-center gap-2">
            <CheckCircle className="w-4 h-4" />
            已通过
          </TabsTrigger>
          <TabsTrigger value="rejected" className="flex items-center gap-2">
            <XCircle className="w-4 h-4" />
            已拒绝
          </TabsTrigger>
        </TabsList>

        <TabsContent value="pending" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <AlertCircle className="w-5 h-5 text-yellow-500" />
                待审核文章
              </CardTitle>
            </CardHeader>
            <CardContent>
              {pendingPosts.length === 0 ? (
                <div className="text-center py-12">
                  <Clock className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                  <p className="text-muted-foreground">暂无待审核的文章</p>
                </div>
              ) : (
                <div className="space-y-4">
                  {pendingPosts.map((post) => (
                    <div 
                      key={post.id} 
                      className="flex items-start justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors"
                    >
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-2">
                          <Badge variant="secondary">待审核</Badge>
                          <span className="text-sm text-muted-foreground">
                            {formatDate(post.createdAt)}
                          </span>
                        </div>
                        <h3 className="font-medium text-lg mb-1">{post.title}</h3>
                        <p className="text-sm text-muted-foreground mb-2">
                          作者: {post.author?.username}
                        </p>
                        {post.excerpt && (
                          <p className="text-sm text-muted-foreground line-clamp-2">
                            {post.excerpt}
                          </p>
                        )}
                      </div>
                      <div className="flex flex-col gap-2 ml-4">
                        <Button
                          size="sm"
                          onClick={() => handleViewPost(post.id)}
                        >
                          <Eye className="w-4 h-4 mr-1" />
                          查看
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => handleEditPost(post.id)}
                        >
                          <Edit className="w-4 h-4 mr-1" />
                          编辑
                        </Button>
                        <div className="flex gap-1">
                          <Button
                            size="sm"
                            variant="default"
                            className="bg-green-600 hover:bg-green-700"
                            onClick={() => handleApprovePost(post.id)}
                          >
                            <CheckCircle className="w-4 h-4 mr-1" />
                            通过
                          </Button>
                          <Button
                            size="sm"
                            variant="destructive"
                            onClick={() => handleRejectPost(post.id)}
                          >
                            <XCircle className="w-4 h-4 mr-1" />
                            拒绝
                          </Button>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="approved" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <CheckCircle className="w-5 h-5 text-green-500" />
                已通过文章
              </CardTitle>
            </CardHeader>
            <CardContent>
              {approvedPosts.length === 0 ? (
                <div className="text-center py-12">
                  <CheckCircle className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                  <p className="text-muted-foreground">暂无已通过的文章</p>
                </div>
              ) : (
                <div className="space-y-4">
                  {approvedPosts.map((post) => (
                    <div
                      key={post.id}
                      className="flex items-start justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors"
                    >
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-2">
                          <Badge variant="default" className="bg-green-500">已通过</Badge>
                          <span className="text-sm text-muted-foreground">
                            {formatDate(post.createdAt)}
                          </span>
                        </div>
                        <h3 className="font-medium text-lg mb-1">{post.title}</h3>
                        <p className="text-sm text-muted-foreground mb-2">
                          作者: {post.author?.username}
                        </p>
                        {post.excerpt && (
                          <p className="text-sm text-muted-foreground line-clamp-2">
                            {post.excerpt}
                          </p>
                        )}
                      </div>
                      <div className="flex flex-col gap-2 ml-4">
                        <Button
                          size="sm"
                          onClick={() => handleViewPost(post.id)}
                        >
                          <Eye className="w-4 h-4 mr-1" />
                          查看
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="rejected" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <XCircle className="w-5 h-5 text-red-500" />
                已拒绝文章
              </CardTitle>
            </CardHeader>
            <CardContent>
              {rejectedPosts.length === 0 ? (
                <div className="text-center py-12">
                  <XCircle className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                  <p className="text-muted-foreground">暂无已拒绝的文章</p>
                </div>
              ) : (
                <div className="space-y-4">
                  {rejectedPosts.map((post) => (
                    <div
                      key={post.id}
                      className="flex items-start justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors"
                    >
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-2">
                          <Badge variant="destructive">已拒绝</Badge>
                          <span className="text-sm text-muted-foreground">
                            {formatDate(post.createdAt)}
                          </span>
                        </div>
                        <h3 className="font-medium text-lg mb-1">{post.title}</h3>
                        <p className="text-sm text-muted-foreground mb-2">
                          作者: {post.author?.username}
                        </p>
                        {post.excerpt && (
                          <p className="text-sm text-muted-foreground line-clamp-2">
                            {post.excerpt}
                          </p>
                        )}
                      </div>
                      <div className="flex flex-col gap-2 ml-4">
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => handleEditPost(post.id)}
                        >
                          <Edit className="w-4 h-4 mr-1" />
                          编辑
                        </Button>
                        <Button
                          size="sm"
                          onClick={() => handleViewPost(post.id)}
                        >
                          <Eye className="w-4 h-4 mr-1" />
                          查看
                        </Button>
                        <Button
                          size="sm"
                          variant="default"
                          className="bg-yellow-600 hover:bg-yellow-700"
                          onClick={() => handleResubmitPost(post.id)}
                        >
                          <Send className="w-4 h-4 mr-1" />
                          重新提交
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}