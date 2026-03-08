import { useEffect, useState } from 'react';
import { Link, useParams, useNavigate } from 'react-router-dom';
import { postsApi, commentsApi, likesApi } from '@/lib/api';
import type { Post, Comment } from '@/types';
import { useAuth } from '@/hooks/useAuth';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';

import { Textarea } from '@/components/ui/textarea';
import { Skeleton } from '@/components/ui/skeleton';
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
} from '@/components/ui/alert-dialog';
import { 
  Heart, MessageCircle, Calendar, 
  ArrowLeft, Share2, Edit, Trash2, Send,
  Clock, Eye, XCircle
} from 'lucide-react';
import { normalizeTagsArray } from '@/lib/utils';

export function PostDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { user, isAuthenticated } = useAuth();
  const [post, setPost] = useState<Post | null>(null);
  const [comments, setComments] = useState<Comment[]>([]);
  const [loading, setLoading] = useState(true);
  const [commentContent, setCommentContent] = useState('');
  const [isLiked, setIsLiked] = useState(false);
  const [likesCount, setLikesCount] = useState(0);
  const [submittingComment, setSubmittingComment] = useState(false);

  useEffect(() => {
    if (id) {
      loadPost();
      loadComments();
      loadLikeStatus();
    }
  }, [id]);

  const loadPost = async () => {
    try {
      console.log('正在加载文章，ID:', id);
      const response = await postsApi.getPost(id!);
      console.log('文章加载成功:', response.data);
      const p = response.data.post;
      p.tags = normalizeTagsArray(p.tags);
      setPost(p);
      setLikesCount(response.data.post.likesCount || 0);
    } catch (error: any) {
      console.error('加载文章失败:', error);
      console.error('错误详情:', {
        message: error.message,
        response: error.response,
        status: error.response?.status,
        data: error.response?.data
      });
      // Only redirect back to the homepage if you truly cannot find the article.
      if (error.response?.status === 404) {
        navigate('/');
      }
      // For other errors, still set loading to false so that users can see the error message.
    } finally {
      setLoading(false);
    }
  };

  const loadComments = async () => {
    try {
      const response = await commentsApi.getComments(id!);
      setComments(response.data.comments);
      return response.data.comments;
    } catch (error) {
      console.error('加载评论失败:', error);
      return [] as Comment[];
    }
  };

  const loadLikeStatus = async () => {
    try {
      const response = await likesApi.getLikeStatus(id!);
      setIsLiked(response.data.isLiked);
      setLikesCount(response.data.likesCount);
    } catch (error) {
      console.error('加载点赞状态失败:', error);
    }
  };

  const handleLike = async () => {
    if (!isAuthenticated) {
      navigate('/login');
      return;
    }

    try {
      const response = await likesApi.toggleLike(id!);
      setIsLiked(response.data.isLiked);
      setLikesCount(response.data.likesCount);
      // 通知其他页面（例如首页）更新对应文章的点赞状态
      try {
        window.dispatchEvent(new CustomEvent('like-changed', { detail: { postId: id, isLiked: response.data.isLiked, likesCount: response.data.likesCount } }));
      } catch (e) {
        // ignore
      }
    } catch (error) {
      console.error('点赞失败:', error);
    }
  };

  const handleSubmitComment = async () => {
    if (!isAuthenticated) {
      navigate('/login');
      return;
    }

    if (!commentContent.trim()) return;

    setSubmittingComment(true);
    try {
      const response = await commentsApi.createComment({
        postId: id!,
        content: commentContent.trim()
      });
      setCommentContent('');
      await loadComments();
      // 使用后端返回的 commentsCount 同步首页
      const newCount = response.data.commentsCount ?? (comments.length + 1);
      try {
        window.dispatchEvent(new CustomEvent('comment-changed', { detail: { postId: id, commentsCount: newCount } }));
      } catch (e) {
        // ignore
      }
    } catch (error) {
      console.error('发表评论失败:', error);
    } finally {
      setSubmittingComment(false);
    }
  };

  const handleDeletePost = async () => {
    try {
      await postsApi.deletePost(id!);
      navigate('/');
    } catch (error) {
      console.error('删除文章失败:', error);
    }
  };

  const handleDeleteComment = async (commentId: string) => {
    try {
      const response = await commentsApi.deleteComment(commentId);
      await loadComments();
      const newCount = response.data.commentsCount ?? (comments.length > 0 ? comments.length - 1 : 0);
      try {
        window.dispatchEvent(new CustomEvent('comment-changed', { detail: { postId: id, commentsCount: newCount } }));
      } catch (e) {}
    } catch (error) {
      console.error('删除评论失败:', error);
    }
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: 'long',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  };

  const canEditPost = user && post && (user.id === post.authorId || user.role === 'admin' || user.role === 'super_admin');
  const canDeletePost = user && post && (user.id === post.authorId || user.role === 'admin' || user.role === 'super_admin');

  if (loading) {
    return (
      <div className="container mx-auto px-4 py-8 max-w-4xl">
        <Skeleton className="h-8 w-3/4 mb-4" />
        <Skeleton className="h-4 w-1/2 mb-8" />
        <Skeleton className="h-64 w-full mb-8" />
        <Skeleton className="h-4 w-full mb-2" />
        <Skeleton className="h-4 w-full mb-2" />
        <Skeleton className="h-4 w-2/3" />
      </div>
    );
  }

  if (!post) {
    return (
      <div className="container mx-auto px-4 py-16 text-center">
        <h1 className="text-2xl font-bold mb-4">文章不存在</h1>
        <Button asChild>
          <Link to="/">返回首页</Link>
        </Button>
      </div>
    );
  }

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl">
      {/* 返回按钮 */}
      <Button variant="ghost" size="sm" asChild className="mb-6">
        <Link to="/" className="flex items-center gap-2">
          <ArrowLeft className="w-4 h-4" />
          返回首页
        </Link>
      </Button>

      {/* 文章头部 */}
      <div className="mb-8">
        {/* 文章状态和拒绝原因 */}
        {post.status === 'rejected' && (
          <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-md">
            <div className="flex items-center gap-2 mb-1">
              <XCircle className="w-5 h-5 text-red-500" />
              <span className="font-medium text-red-800">此文章已被拒绝</span>
            </div>
            {post.rejectionReason && (
              <p className="text-sm text-red-700">
                拒绝原因：{post.rejectionReason}
              </p>
            )}
          </div>
        )}
        
        {/* 分类和标签 */}
        <div className="flex flex-wrap items-center gap-2 mb-4">
          {post.categoryInfo && (
            <Badge variant="secondary">
              {post.categoryInfo.name}
            </Badge>
          )}
          {post.tags?.map((tag) => (
            <Badge key={tag} variant="outline">
              {tag}
            </Badge>
          ))}
        </div>

        {/* 标题 */}
        <h1 className="text-3xl md:text-4xl font-bold mb-6">
          {post.title}
        </h1>

        {/* 作者信息 */}
        <div className="flex items-center justify-between flex-wrap gap-4">
          <Link to={`/user/${post.author?.id}`} className="flex items-center gap-4 hover:no-underline">
            <Avatar className="w-12 h-12">
              {post.author?.avatar ? (
                <img src={post.author.avatar} alt="Avatar" className="w-full h-full object-cover" />
              ) : (
                <AvatarFallback className="bg-primary/10 text-primary text-lg">
                  {post.author?.username?.charAt(0).toUpperCase()}
                </AvatarFallback>
              )}
            </Avatar>
            <div>
              <p className="font-medium hover:text-primary transition-colors">{post.author?.username}</p>
              <div className="flex items-center gap-3 text-sm text-muted-foreground">
                <span className="flex items-center gap-1">
                  <Calendar className="w-4 h-4" />
                  {formatDate(post.createdAt)}
                </span>
                {post.updatedAt !== post.createdAt && (
                  <span className="flex items-center gap-1">
                    <Clock className="w-4 h-4" />
                    更新于 {formatDate(post.updatedAt)}
                  </span>
                )}
              </div>
            </div>
          </Link>

          {/* 操作按钮 */}
          <div className="flex items-center gap-2">
            {canEditPost && (
              <Button variant="outline" size="sm" asChild>
                <Link to={`/edit-post/${post.id}`}>
                  <Edit className="w-4 h-4 mr-1" />
                  编辑
                </Link>
              </Button>
            )}
            {canDeletePost && (
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button variant="destructive" size="sm">
                    <Trash2 className="w-4 h-4 mr-1" />
                    删除
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>确认删除文章？</AlertDialogTitle>
                    <AlertDialogDescription>
                      此操作不可撤销，文章及其所有评论将被永久删除。
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel>取消</AlertDialogCancel>
                    <AlertDialogAction onClick={handleDeletePost} className="bg-destructive">
                      删除
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            )}
          </div>
        </div>
      </div>

      {/* 封面图 */}
      {post.coverImage && (
        <div className="mb-8 rounded-lg overflow-hidden">
          <img
            src={post.coverImage}
            alt={post.title}
            className="w-full max-h-[400px] object-fill"
          />
        </div>
      )}

      {/* 文章内容 */}
      <div 
        className="prose prose-lg max-w-none mb-12"
        dangerouslySetInnerHTML={{ __html: post.content }}
      />

      {/* 互动区域 */}
      <div className="flex items-center justify-between py-6 border-y">
        <div className="flex items-center gap-4">
          <Button
            variant={isLiked ? 'default' : 'outline'}
            size="lg"
            onClick={handleLike}
            className="flex items-center gap-2"
          >
            <Heart className={`w-5 h-5 ${isLiked ? 'fill-current' : ''}`} />
            <span>{likesCount}</span>
          </Button>
          <Button
            variant="outline"
            size="lg"
            className="flex items-center gap-2"
            onClick={() => document.getElementById('comments')?.scrollIntoView({ behavior: 'smooth' })}
          >
            <MessageCircle className="w-5 h-5" />
            <span>{comments.length}</span>
          </Button>
          <div className="flex items-center gap-1 text-sm text-muted-foreground">
            <Eye className="w-5 h-5" />
            <span>{post.viewCount || 0}</span>
          </div>
        </div>
        <Button variant="ghost" size="icon">
          <Share2 className="w-5 h-5" />
        </Button>
      </div>

      {/* 评论区 */}
      <div id="comments" className="mt-12">
        <h2 className="text-2xl font-bold mb-6 flex items-center gap-2">
          <MessageCircle className="w-6 h-6" />
          评论 ({comments.length})
        </h2>

        {/* 发表评论 */}
        {isAuthenticated ? (
          <Card className="mb-8">
            <CardContent className="p-4">
              <Textarea
                placeholder="写下你的评论..."
                value={commentContent}
                onChange={(e) => setCommentContent(e.target.value)}
                className="mb-4 min-h-[100px]"
              />
              <div className="flex justify-end">
                <Button 
                  onClick={handleSubmitComment}
                  disabled={!commentContent.trim() || submittingComment}
                >
                  <Send className="w-4 h-4 mr-2" />
                  {submittingComment ? '发送中...' : '发表评论'}
                </Button>
              </div>
            </CardContent>
          </Card>
        ) : (
          <Card className="mb-8">
            <CardContent className="p-6 text-center">
              <p className="text-muted-foreground mb-4">登录后可以发表评论</p>
              <Button asChild>
                <Link to="/login">登录</Link>
              </Button>
            </CardContent>
          </Card>
        )}

        {/* 评论列表 */}
        <div className="space-y-4">
          {comments.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground">
              暂无评论，来发表第一条评论吧！
            </div>
          ) : (
            comments.map((comment) => (
              <Card key={comment.id}>
                <CardContent className="p-4">
                  <div className="flex items-start gap-4">
                    <Avatar className="w-10 h-10">
                      {comment.author?.avatar ? (
                        <img src={comment.author.avatar} alt="Avatar" className="w-full h-full object-cover" />
                      ) : (
                        <AvatarFallback className="bg-primary/10 text-primary text-sm">
                          {comment.author?.username?.charAt(0).toUpperCase()}
                        </AvatarFallback>
                      )}
                    </Avatar>
                    <div className="flex-1">
                      <div className="flex items-center justify-between mb-2">
                        <div className="flex items-center gap-2">
                          <span className="font-medium">{comment.author?.username}</span>
                          <span className="text-sm text-muted-foreground">
                            {formatDate(comment.createdAt)}
                          </span>
                        </div>
                        {user && (user.id === comment.userId || user.role === 'admin' || user.role === 'super_admin') && (
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => handleDeleteComment(comment.id)}
                          >
                            <Trash2 className="w-4 h-4" />
                          </Button>
                        )}
                      </div>
                      <p className="text-gray-700">{comment.content}</p>
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))
          )}
        </div>
      </div>
    </div>
  );
}