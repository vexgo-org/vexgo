import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from '@/lib/I18nContext';
import { getLocale } from '@/lib/i18n';
import { getPendingPosts, getApprovedPosts, getRejectedPosts, approvePost, rejectPost, resubmitPost } from '@/lib/moderationApi';
import type { Post } from '@/types';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Textarea } from '@/components/ui/textarea';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import {
  CheckCircle, XCircle, Clock, AlertCircle, Search,
  Eye, Edit, Send, MessageSquare
} from 'lucide-react';
import { toast } from 'sonner';

export function ModerationPage() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [pendingPosts, setPendingPosts] = useState<Post[]>([]);
  const [approvedPosts, setApprovedPosts] = useState<Post[]>([]);
  const [rejectedPosts, setRejectedPosts] = useState<Post[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState('pending');
  const [rejectingPostId, setRejectingPostId] = useState<string | null>(null);
  const [rejectionReason, setRejectionReason] = useState('');
  const [showRejectDialog, setShowRejectDialog] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');

  useEffect(() => {
    loadData();
  }, [activeTab]);

  const loadData = async (search?: string) => {
    setLoading(true);
    try {
      // 确保只更新当前标签页的数据，避免其他标签页数据被清空
      if (activeTab === 'pending') {
        const response = await getPendingPosts({ limit: 100, search: search });
        if (response && response.data) {
          setPendingPosts(response.data.posts || []);
        } else {
          setPendingPosts([]);
        }
      } else if (activeTab === 'approved') {
        const response = await getApprovedPosts({ limit: 100, search: search });
        if (response && response.data) {
          setApprovedPosts(response.data.posts || []);
        } else {
          setApprovedPosts([]);
        }
      } else if (activeTab === 'rejected') {
        const response = await getRejectedPosts({ limit: 100, search: search });
        if (response && response.data) {
          setRejectedPosts(response.data.posts || []);
        } else {
          setRejectedPosts([]);
        }
      }
    } catch (error) {
      console.error('加载数据失败:', error);
      toast.error(t('moderation.loadFailed'));
      // 出错时保持当前数据不变，避免白屏
    } finally {
      setLoading(false);
    }
  };

  const handleSearch = async () => {
    // 直接调用loadData函数，传递搜索参数
    await loadData(searchTerm);
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleSearch();
    }
  };

  // 当切换标签页时，清空搜索
  const handleTabChange = (tab: string) => {
    setActiveTab(tab);
    setSearchTerm('');
  };

  const handleApprovePost = async (postId: string) => {
    try {
      await approvePost(postId);
      toast.success(t('moderation.approveSuccess'));
      loadData();
    } catch (error) {
      console.error('审核通过失败:', error);
      toast.error(t('moderation.approveFailed'));
    }
  };

  const handleRejectPost = async (postId: string) => {
    setRejectingPostId(postId);
    setShowRejectDialog(true);
    setRejectionReason('');
  };

  const confirmRejectPost = async () => {
    if (!rejectingPostId) return;

    try {
      await rejectPost(rejectingPostId, rejectionReason);
      toast.success(t('moderation.rejectSuccess'));
      setShowRejectDialog(false);
      setRejectingPostId(null);
      setRejectionReason('');
      loadData();
    } catch (error) {
      console.error('拒绝文章失败:', error);
      toast.error(t('moderation.rejectFailed'));
    }
  };

  const cancelRejectPost = () => {
    setShowRejectDialog(false);
    setRejectingPostId(null);
    setRejectionReason('');
  };

  const handleResubmitPost = async (postId: string) => {
    try {
      await resubmitPost(postId);
      toast.success(t('moderation.resubmitSuccess'));
      loadData();
    } catch (error) {
      console.error('重新提交审核失败:', error);
      toast.error(t('moderation.resubmitFailed'));
    }
  };

  const handleViewPost = (postId: string) => {
    navigate(`/post/${postId}`);
  };

  const handleEditPost = (postId: string) => {
    navigate(`/edit-post/${postId}`);
  };

  const formatDate = (dateString: string) => {
    const locale = getLocale();
    return new Date(dateString).toLocaleDateString(locale, {
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
          <h1 className="text-2xl font-bold">{t('moderation.title')}</h1>
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
        <h1 className="text-2xl font-bold">{t('moderation.title')}</h1>
        <div className="relative w-64">
          <Input
            type="text"
            placeholder={t('moderation.searchPlaceholder')}
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            onKeyPress={handleKeyPress}
            className="pl-10"
          />
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-muted-foreground w-4 h-4" />
          <Button
            variant="ghost"
            size="sm"
            className="absolute right-2 top-1/2 transform -translate-y-1/2 h-8 w-8 p-0"
            onClick={handleSearch}
          >
            <Search className="w-4 h-4" />
          </Button>
        </div>
      </div>

      <Tabs value={activeTab} onValueChange={handleTabChange} className="w-full">
        <TabsList className="grid w-full grid-cols-3 max-w-md">
          <TabsTrigger value="pending" className="flex items-center gap-2">
            <Clock className="w-4 h-4" />
            {t('moderation.pending')} ({pendingPosts.length})
          </TabsTrigger>
          <TabsTrigger value="approved" className="flex items-center gap-2">
            <CheckCircle className="w-4 h-4" />
            {t('moderation.approved')}
          </TabsTrigger>
          <TabsTrigger value="rejected" className="flex items-center gap-2">
            <XCircle className="w-4 h-4" />
            {t('moderation.rejected')}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="pending" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <AlertCircle className="w-5 h-5 text-yellow-500" />
                {t('moderation.pendingPosts')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {pendingPosts.length === 0 ? (
                <div className="text-center py-12">
                  <Clock className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                  <p className="text-muted-foreground">{t('moderation.noPendingPosts')}</p>
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
                          <Badge variant="secondary">{t('moderation.pending')}</Badge>
                          <span className="text-sm text-muted-foreground">
                            {formatDate(post.createdAt)}
                          </span>
                        </div>
                        <h3 className="font-medium text-lg mb-1">{post.title}</h3>
                        <p className="text-sm text-muted-foreground mb-2">
                          {t('moderation.author')}: {post.author?.username}
                        </p>
                        {post.excerpt && (
                          <p className="text-sm text-muted-foreground mb-2 line-clamp-2">
                            {post.excerpt}
                          </p>
                        )}
                        {post.rejectionReason && (
                          <div className="mt-2 p-2 bg-red-50 border border-red-200 rounded">
                            <p className="text-sm text-red-800">
                              <span className="font-medium">{t('moderation.rejectionReasonInPost')}</span>
                              {post.rejectionReason}
                            </p>
                          </div>
                        )}
                      </div>
                      <div className="flex flex-col gap-2 ml-4">
                        <Button
                          size="sm"
                          onClick={() => handleViewPost(post.id)}
                        >
                          <Eye className="w-4 h-4 mr-1" />
                          {t('moderation.view')}
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => handleEditPost(post.id)}
                        >
                          <Edit className="w-4 h-4 mr-1" />
                          {t('moderation.edit')}
                        </Button>
                        <div className="flex gap-1">
                          <Button
                            size="sm"
                            variant="default"
                            className="bg-green-600 hover:bg-green-700"
                            onClick={() => handleApprovePost(post.id)}
                          >
                            <CheckCircle className="w-4 h-4 mr-1" />
                            {t('moderation.approve')}
                          </Button>
                          <Button
                            size="sm"
                            variant="destructive"
                            onClick={() => handleRejectPost(post.id)}
                          >
                            <XCircle className="w-4 h-4 mr-1" />
                            {t('moderation.reject')}
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
                {t('moderation.approvedPosts')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {approvedPosts.length === 0 ? (
                <div className="text-center py-12">
                  <CheckCircle className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                  <p className="text-muted-foreground">{t('moderation.noApprovedPosts')}</p>
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
                          <Badge variant="default" className="bg-green-500">{t('moderation.approved')}</Badge>
                          <span className="text-sm text-muted-foreground">
                            {formatDate(post.createdAt)}
                          </span>
                        </div>
                        <h3 className="font-medium text-lg mb-1">{post.title}</h3>
                        <p className="text-sm text-muted-foreground mb-2">
                          {t('moderation.author')}: {post.author?.username}
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
                          {t('moderation.view')}
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
                {t('moderation.rejectedPosts')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {rejectedPosts.length === 0 ? (
                <div className="text-center py-12">
                  <XCircle className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                  <p className="text-muted-foreground">{t('moderation.noRejectedPosts')}</p>
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
                          <Badge variant="destructive">{t('moderation.rejected')}</Badge>
                          <span className="text-sm text-muted-foreground">
                            {formatDate(post.createdAt)}
                          </span>
                        </div>
                        <h3 className="font-medium text-lg mb-1">{post.title}</h3>
                        <p className="text-sm text-muted-foreground mb-2">
                          {t('moderation.author')}: {post.author?.username}
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
                          {t('moderation.edit')}
                        </Button>
                        <Button
                          size="sm"
                          onClick={() => handleViewPost(post.id)}
                        >
                          <Eye className="w-4 h-4 mr-1" />
                          {t('moderation.view')}
                        </Button>
                        <Button
                          size="sm"
                          variant="default"
                          className="bg-yellow-600 hover:bg-yellow-700"
                          onClick={() => handleResubmitPost(post.id)}
                        >
                          <Send className="w-4 h-4 mr-1" />
                          {t('moderation.resubmit')}
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

      {/* 拒绝原因对话框 */}
      {showRejectDialog && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-md">
            <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
              <MessageSquare className="w-5 h-5 text-red-500" />
              {t('moderation.rejectPost')}
            </h2>
            <div className="mb-4">
              <Label htmlFor="rejectionReason" className="block text-sm font-medium mb-2">
                {t('moderation.rejectionReasonLabel')}
              </Label>
              <Textarea
                id="rejectionReason"
                value={rejectionReason}
                onChange={(e) => setRejectionReason(e.target.value)}
                placeholder={t('moderation.rejectionReasonPlaceholder')}
                rows={4}
                className="w-full"
              />
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={cancelRejectPost}>
                {t('moderation.cancel')}
              </Button>
              <Button variant="destructive" onClick={confirmRejectPost}>
                {t('moderation.confirmReject')}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}