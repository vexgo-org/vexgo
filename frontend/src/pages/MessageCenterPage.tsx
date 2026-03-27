import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Badge } from '@/components/ui/badge';
import { messagesApi } from '@/lib/api';
import { useTranslation } from '@/lib/I18nContext';
import { CreatorApplicationButton } from '@/components/CreatorApplicationButton';

import {
  Bell,
  MessageSquare,
  AlertCircle,
  UserPlus,
  CheckCircle,
  ThumbsUp,
  FileText,
  ArrowRight,
  Trash2,
  Eye,
  Users,
} from 'lucide-react';

// 消息类型
type MessageType = 'comment' | 'like' | 'reply' | 'review' | 'role';

type Message = {
  id: string;
  type: MessageType;
  title: string;
  content: string;
  relatedId: string;
  relatedType: 'post' | 'comment';
  createdAt: string;
  isRead: boolean;
  sender?: {
    id: string;
    username: string;
    avatar?: string;
  };
};

export function MessageCenterPage() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState('all');
  
  // 消息数据
  const [messages, setMessages] = useState<Message[]>([]);
  
  const [loading, setLoading] = useState(false);

  // 检查是否是管理员
  const isAdmin = user?.role === 'admin' || user?.role === 'super_admin';
  // 检查是否是访客
  const isGuest = user?.role === 'guest';

  // 从API获取消息数据
  useEffect(() => {
    const fetchMessages = async () => {
      setLoading(true);
      try {
        const response = await messagesApi.getMessages();
        // 转换后端数据格式为前端使用的格式
        interface Notification {
          id: number;
          type: string;
          title: string;
          content: string;
          related_id: string;
          related_type: 'post' | 'comment';
          created_at: string;
          is_read: boolean;
        }
        
        const formattedMessages = response.data.notifications.map((notification: Notification) => ({
          id: notification.id.toString(),
          type: notification.type as MessageType,
          title: notification.title,
          content: notification.content,
          relatedId: notification.related_id,
          relatedType: notification.related_type,
          createdAt: notification.created_at,
          isRead: notification.is_read,
          // 后端数据中可能没有sender信息，这里暂时为空
          sender: undefined
        }));
        setMessages(formattedMessages);
      } catch (error) {
        console.error(t('errors.networkError'), error);
      } finally {
        setLoading(false);
      }
    };

    fetchMessages();
  }, []);

  // 根据标签筛选消息
  const filteredMessages = messages.filter((message) => {
    if (activeTab === 'all') return true;
    if (activeTab === 'unread') return !message.isRead;
    if (activeTab === 'comment') return message.type === 'comment' || message.type === 'reply';
    if (activeTab === 'like') return message.type === 'like';
    if (activeTab === 'review') return message.type === 'review';
    if (activeTab === 'role') return message.type === 'role';
    return true;
  });

  // 标记消息为已读
  const markAsRead = async (id: string) => {
    try {
      await messagesApi.markAsRead(id);
      // 更新本地状态
      setMessages((prev) =>
        prev.map((message) =>
          message.id === id ? { ...message, isRead: true } : message
        )
      );
    } catch (error) {
      console.error(t('errors.networkError'), error);
    }
  };

  // 全部标记为已读
  const markAllAsRead = async () => {
    try {
      await messagesApi.markAllAsRead();
      // 更新本地状态
      setMessages((prev) =>
        prev.map((message) => ({ ...message, isRead: true }))
      );
    } catch (error) {
      console.error(t('errors.networkError'), error);
    }
  };

  // 删除消息
  const deleteMessage = async (id: string) => {
    try {
      await messagesApi.deleteMessage(id);
      // 更新本地状态
      setMessages((prev) => prev.filter((message) => message.id !== id));
    } catch (error) {
      console.error(t('errors.networkError'), error);
    }
  };

  // 跳转到相关内容
  const navigateToRelated = (relatedId: string, relatedType: 'post' | 'comment') => {
    if (relatedType === 'post') {
      navigate(`/post/${relatedId}`);
    } else if (relatedType === 'comment') {
      // 跳转到文章页面并定位到评论
      navigate(`/post/123#comment-${relatedId}`);
    }
  };

  // 获取消息状态图标
  const getStatusIcon = (type: MessageType) => {
    switch (type) {
      case 'review':
        return <CheckCircle className="w-4 h-4 text-green-500" />;
      case 'role':
        return <CheckCircle className="w-4 h-4 text-green-500" />;
      default:
        return null;
    }
  };

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl">
      <div className="flex items-center justify-between mb-8">
        <h1 className="text-2xl font-bold">{t('messageCenter.title')}</h1>
        <div className="flex items-center gap-2">
          {isGuest && (
            <CreatorApplicationButton />
          )}
          {isAdmin && (
            <Button 
              variant="outline" 
              size="sm" 
              onClick={() => navigate('/creator-applications')}
            >
              <Users className="w-4 h-4 mr-2" />
              {t('creatorApplication.reviewApplications')}
            </Button>
          )}
          <Button variant="outline" size="sm" onClick={markAllAsRead}>
            {t('messageCenter.markAllAsRead')}
          </Button>
        </div>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
        <TabsList className="mb-6">
          <TabsTrigger value="all">{t('messageCenter.tabs.all')}</TabsTrigger>
          <TabsTrigger value="unread">{t('messageCenter.tabs.unread')}</TabsTrigger>
          <TabsTrigger value="comment">{t('messageCenter.tabs.comment')}</TabsTrigger>
          <TabsTrigger value="like">{t('messageCenter.tabs.like')}</TabsTrigger>
          <TabsTrigger value="review">{t('messageCenter.tabs.review')}</TabsTrigger>
          <TabsTrigger value="role">{t('messageCenter.tabs.role')}</TabsTrigger>
        </TabsList>

        <TabsContent value="all" className="space-y-4">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary"></div>
            </div>
          ) : filteredMessages.length === 0 ? (
            <Card>
              <CardContent className="py-12 text-center">
                <Bell className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                <h3 className="text-lg font-medium mb-2">{t('messageCenter.empty.noMessages')}</h3>
                <p className="text-muted-foreground">{t('messageCenter.empty.noMessagesDesc')}</p>
              </CardContent>
            </Card>
          ) : (
            filteredMessages.map((message) => (
              <Card
                key={message.id}
                className={`cursor-pointer transition-all duration-200 hover:shadow-md ${message.isRead ? '' : 'border-l-4 border-primary'}`}
                onClick={() => {
                  markAsRead(message.id);
                  if (message.relatedId) {
                    navigateToRelated(message.relatedId, message.relatedType);
                  }
                }}
              >
                <CardContent className="p-4">
                  <div className="flex items-start gap-4">
                    <div className="flex-shrink-0">
                      {message.sender ? (
                        <Avatar className="h-10 w-10">
                          <AvatarImage src={message.sender.avatar} alt={message.sender.username} />
                          <AvatarFallback>{message.sender.username.charAt(0).toUpperCase()}</AvatarFallback>
                        </Avatar>
                      ) : (
                        <div className="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center">
                          <AlertCircle className="w-5 h-5 text-primary" />
                        </div>
                      )}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between mb-1">
                        <h3 className="font-medium text-sm">{message.title}</h3>
                        <span className="text-xs text-muted-foreground">
                          {new Date(message.createdAt).toLocaleString()}
                        </span>
                      </div>
                      <p className="text-sm text-muted-foreground mb-2">{message.content}</p>
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          {!message.isRead && (
                           <Badge variant="secondary" className="text-xs">{t('messageCenter.unreadBadge')}</Badge>
                         )}
                          {getStatusIcon(message.type)}
                        </div>
                        <div className="flex items-center gap-2">
                          {message.type !== 'role' && (
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-8 px-2"
                              onClick={(e) => {
                                e.stopPropagation();
                                if (message.relatedId) {
                                  navigateToRelated(message.relatedId, message.relatedType);
                                }
                              }}
                            >
                              {t('messageCenter.view')} <ArrowRight className="w-3 h-3 ml-1" />
                            </Button>
                          )}
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8 text-destructive"
                            onClick={(e) => {
                              e.stopPropagation();
                              deleteMessage(message.id);
                            }}
                          >
                            <Trash2 className="w-4 h-4" />
                          </Button>
                        </div>
                      </div>
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))
          )}
        </TabsContent>

        {/* 其他标签内容 */}
        <TabsContent value="unread" className="space-y-4">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary"></div>
            </div>
          ) : filteredMessages.length === 0 ? (
            <Card>
              <CardContent className="py-12 text-center">
                <Eye className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                <h3 className="text-lg font-medium mb-2">{t('messageCenter.empty.noUnreadMessages')}</h3>
                <p className="text-muted-foreground">{t('messageCenter.empty.allRead')}</p>
              </CardContent>
            </Card>
          ) : (
            filteredMessages.map((message) => (
              <Card
                key={message.id}
                className="border-l-4 border-primary cursor-pointer transition-all duration-200 hover:shadow-md"
                onClick={() => {
                  markAsRead(message.id);
                  if (message.relatedId) {
                    navigateToRelated(message.relatedId, message.relatedType);
                  }
                }}
              >
                <CardContent className="p-4">
                  <div className="flex items-start gap-4">
                    <div className="flex-shrink-0">
                      {message.sender ? (
                        <Avatar className="h-10 w-10">
                          <AvatarImage src={message.sender.avatar} alt={message.sender.username} />
                          <AvatarFallback>{message.sender.username.charAt(0).toUpperCase()}</AvatarFallback>
                        </Avatar>
                      ) : (
                        <div className="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center">
                          <AlertCircle className="w-5 h-5 text-primary" />
                        </div>
                      )}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between mb-1">
                        <h3 className="font-medium text-sm">{message.title}</h3>
                        <span className="text-xs text-muted-foreground">
                          {new Date(message.createdAt).toLocaleString()}
                        </span>
                      </div>
                      <p className="text-sm text-muted-foreground mb-2">{message.content}</p>
                      <div className="flex items-center justify-between">
                        <Badge variant="secondary" className="text-xs">{t('messageCenter.unreadBadge')}</Badge>
                        <div className="flex items-center gap-2">
                          {message.type !== 'role' && (
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-8 px-2"
                              onClick={(e) => {
                                e.stopPropagation();
                                if (message.relatedId) {
                                  navigateToRelated(message.relatedId, message.relatedType);
                                }
                              }}
                            >
                              {t('messageCenter.view')} <ArrowRight className="w-3 h-3 ml-1" />
                            </Button>
                          )}
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8 text-destructive"
                            onClick={(e) => {
                              e.stopPropagation();
                              deleteMessage(message.id);
                            }}
                          >
                            <Trash2 className="w-4 h-4" />
                          </Button>
                        </div>
                      </div>
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))
          )}
        </TabsContent>

        <TabsContent value="comment" className="space-y-4">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary"></div>
            </div>
          ) : filteredMessages.length === 0 ? (
            <Card>
              <CardContent className="py-12 text-center">
                <MessageSquare className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                <h3 className="text-lg font-medium mb-2">{t('messageCenter.empty.noCommentMessages')}</h3>
                <p className="text-muted-foreground">{t('messageCenter.empty.noCommentMessagesDesc')}</p>
              </CardContent>
            </Card>
          ) : (
            filteredMessages.map((message) => (
              <Card
                key={message.id}
                className={`cursor-pointer transition-all duration-200 hover:shadow-md ${message.isRead ? '' : 'border-l-4 border-primary'}`}
                onClick={() => {
                  markAsRead(message.id);
                  if (message.relatedId) {
                    navigateToRelated(message.relatedId, message.relatedType);
                  }
                }}
              >
                <CardContent className="p-4">
                  <div className="flex items-start gap-4">
                    <div className="flex-shrink-0">
                      {message.sender ? (
                        <Avatar className="h-10 w-10">
                          <AvatarImage src={message.sender.avatar} alt={message.sender.username} />
                          <AvatarFallback>{message.sender.username.charAt(0).toUpperCase()}</AvatarFallback>
                        </Avatar>
                      ) : (
                        <div className="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center">
                          <MessageSquare className="w-5 h-5 text-primary" />
                        </div>
                      )}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between mb-1">
                        <h3 className="font-medium text-sm">{message.title}</h3>
                        <span className="text-xs text-muted-foreground">
                          {new Date(message.createdAt).toLocaleString()}
                        </span>
                      </div>
                      <p className="text-sm text-muted-foreground mb-2">{message.content}</p>
                      <div className="flex items-center justify-between">
                        {!message.isRead && (
                          <Badge variant="secondary" className="text-xs">{t('messageCenter.unreadBadge')}</Badge>
                        )}
                        <div className="flex items-center gap-2">
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-8 px-2"
                            onClick={(e) => {
                              e.stopPropagation();
                              if (message.relatedId) {
                                navigateToRelated(message.relatedId, message.relatedType);
                              }
                            }}
                          >
                            {t('messageCenter.view')} <ArrowRight className="w-3 h-3 ml-1" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8 text-destructive"
                            onClick={(e) => {
                              e.stopPropagation();
                              deleteMessage(message.id);
                            }}
                          >
                            <Trash2 className="w-4 h-4" />
                          </Button>
                        </div>
                      </div>
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))
          )}
        </TabsContent>

        <TabsContent value="like" className="space-y-4">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary"></div>
            </div>
          ) : filteredMessages.length === 0 ? (
            <Card>
              <CardContent className="py-12 text-center">
                <ThumbsUp className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                <h3 className="text-lg font-medium mb-2">{t('messageCenter.empty.noLikeMessages')}</h3>
                <p className="text-muted-foreground">{t('messageCenter.empty.noLikeMessagesDesc')}</p>
              </CardContent>
            </Card>
          ) : (
            filteredMessages.map((message) => (
              <Card
                key={message.id}
                className={`cursor-pointer transition-all duration-200 hover:shadow-md ${message.isRead ? '' : 'border-l-4 border-primary'}`}
                onClick={() => {
                  markAsRead(message.id);
                  if (message.relatedId) {
                    navigateToRelated(message.relatedId, message.relatedType);
                  }
                }}
              >
                <CardContent className="p-4">
                  <div className="flex items-start gap-4">
                    <div className="flex-shrink-0">
                      {message.sender ? (
                        <Avatar className="h-10 w-10">
                          <AvatarImage src={message.sender.avatar} alt={message.sender.username} />
                          <AvatarFallback>{message.sender.username.charAt(0).toUpperCase()}</AvatarFallback>
                        </Avatar>
                      ) : (
                        <div className="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center">
                          <ThumbsUp className="w-5 h-5 text-primary" />
                        </div>
                      )}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between mb-1">
                        <h3 className="font-medium text-sm">{message.title}</h3>
                        <span className="text-xs text-muted-foreground">
                          {new Date(message.createdAt).toLocaleString()}
                        </span>
                      </div>
                      <p className="text-sm text-muted-foreground mb-2">{message.content}</p>
                      <div className="flex items-center justify-between">
                        {!message.isRead && (
                          <Badge variant="secondary" className="text-xs">{t('messageCenter.unreadBadge')}</Badge>
                        )}
                        <div className="flex items-center gap-2">
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-8 px-2"
                            onClick={(e) => {
                              e.stopPropagation();
                              if (message.relatedId) {
                                navigateToRelated(message.relatedId, message.relatedType);
                              }
                            }}
                          >
                            {t('messageCenter.view')} <ArrowRight className="w-3 h-3 ml-1" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8 text-destructive"
                            onClick={(e) => {
                              e.stopPropagation();
                              deleteMessage(message.id);
                            }}
                          >
                            <Trash2 className="w-4 h-4" />
                          </Button>
                        </div>
                      </div>
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))
          )}
        </TabsContent>

        <TabsContent value="review" className="space-y-4">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary"></div>
            </div>
          ) : filteredMessages.length === 0 ? (
            <Card>
              <CardContent className="py-12 text-center">
                <FileText className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                <h3 className="text-lg font-medium mb-2">{t('messageCenter.empty.noReviewMessages')}</h3>
                <p className="text-muted-foreground">{t('messageCenter.empty.noReviewMessagesDesc')}</p>
              </CardContent>
            </Card>
          ) : (
            filteredMessages.map((message) => (
              <Card
                key={message.id}
                className={`cursor-pointer transition-all duration-200 hover:shadow-md ${message.isRead ? '' : 'border-l-4 border-primary'}`}
                onClick={() => {
                  markAsRead(message.id);
                  if (message.relatedId) {
                    navigateToRelated(message.relatedId, message.relatedType);
                  }
                }}
              >
                <CardContent className="p-4">
                  <div className="flex items-start gap-4">
                    <div className="flex-shrink-0">
                      <div className="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center">
                        <FileText className="w-5 h-5 text-primary" />
                      </div>
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between mb-1">
                        <h3 className="font-medium text-sm">{message.title}</h3>
                        <span className="text-xs text-muted-foreground">
                          {new Date(message.createdAt).toLocaleString()}
                        </span>
                      </div>
                      <p className="text-sm text-muted-foreground mb-2">{message.content}</p>
                      <div className="flex items-center justify-between">
                        {!message.isRead && (
                          <Badge variant="secondary" className="text-xs">{t('messageCenter.unreadBadge')}</Badge>
                        )}
                        <div className="flex items-center gap-2">
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-8 px-2"
                            onClick={(e) => {
                              e.stopPropagation();
                              if (message.relatedId) {
                                navigateToRelated(message.relatedId, message.relatedType);
                              }
                            }}
                          >
                            {t('messageCenter.view')} <ArrowRight className="w-3 h-3 ml-1" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8 text-destructive"
                            onClick={(e) => {
                              e.stopPropagation();
                              deleteMessage(message.id);
                            }}
                          >
                            <Trash2 className="w-4 h-4" />
                          </Button>
                        </div>
                      </div>
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))
          )}
        </TabsContent>

        <TabsContent value="role" className="space-y-4">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary"></div>
            </div>
          ) : filteredMessages.length === 0 ? (
            <Card>
              <CardContent className="py-12 text-center">
                <UserPlus className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                <h3 className="text-lg font-medium mb-2">{t('messageCenter.empty.noRoleMessages')}</h3>
                <p className="text-muted-foreground">{t('messageCenter.empty.noRoleMessagesDesc')}</p>
              </CardContent>
            </Card>
          ) : (
            filteredMessages.map((message) => (
              <Card
                key={message.id}
                className={`cursor-pointer transition-all duration-200 hover:shadow-md ${message.isRead ? '' : 'border-l-4 border-primary'}`}
                onClick={() => {
                  markAsRead(message.id);
                }}
              >
                <CardContent className="p-4">
                  <div className="flex items-start gap-4">
                    <div className="flex-shrink-0">
                      <div className="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center">
                        <UserPlus className="w-5 h-5 text-primary" />
                      </div>
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between mb-1">
                        <h3 className="font-medium text-sm">{message.title}</h3>
                        <span className="text-xs text-muted-foreground">
                          {new Date(message.createdAt).toLocaleString()}
                        </span>
                      </div>
                      <p className="text-sm text-muted-foreground mb-2">{message.content}</p>
                      <div className="flex items-center justify-between">
                        {!message.isRead && (
                          <Badge variant="secondary" className="text-xs">{t('messageCenter.unreadBadge')}</Badge>
                        )}
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 text-destructive"
                          onClick={(e) => {
                            e.stopPropagation();
                            deleteMessage(message.id);
                          }}
                        >
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </div>
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))
          )}
        </TabsContent>
      </Tabs>
    </div>
  );
}