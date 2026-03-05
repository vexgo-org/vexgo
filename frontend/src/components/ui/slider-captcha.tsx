import React, { useState, useEffect, useRef } from 'react';
import { Button } from './button';
import { RefreshCw, CheckCircle, X } from 'lucide-react';

interface SliderCaptchaProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: (captchaData: { id: string; token: string; x: number }) => void;
}

export function SliderCaptcha({ isOpen, onClose, onSuccess }: SliderCaptchaProps) {
  const [captchaData, setCaptchaData] = useState<{
    id: string;
    token: string;
    bg_image: string;
    puzzle_img: string;
    y: number;
    expires_at: string;
  } | null>(null);
  const [isVerifying, setIsVerifying] = useState(false);
  const [isVerified, setIsVerified] = useState(false);
  const [error, setError] = useState('');
  const [isDragging, setIsDragging] = useState(false);
  const [sliderPosition, setSliderPosition] = useState(0);
  const sliderRef = useRef<HTMLDivElement>(null);
  const trackRef = useRef<HTMLDivElement>(null);
  const startXRef = useRef(0);
  const currentXRef = useRef(0);

  // 生成验证码
  const generateCaptcha = async () => {
    try {
      setIsVerifying(true);
      setError('');
      setSliderPosition(0);
      setIsVerified(false);

      const response = await fetch('/api/captcha');
      if (!response.ok) {
        throw new Error('获取验证码失败');
      }

      const data = await response.json();
      setCaptchaData(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : '获取验证码失败');
    } finally {
      setIsVerifying(false);
    }
  };

  // 当弹窗打开时生成验证码
  useEffect(() => {
    if (isOpen) {
      generateCaptcha();
    }
  }, [isOpen]);

  // 验证拼图
  const verifyCaptcha = async (x: number) => {
    if (!captchaData) return;

    try {
      setIsVerifying(true);
      setError('');

      // 计算实际的x坐标（基于后端背景图片宽度320px）
      // 前端轨道宽度可能与后端背景图片宽度不同，需要进行比例转换
      const trackWidth = trackRef.current?.offsetWidth || 320;
      const sliderWidth = 40;
      const maxPosition = trackWidth - sliderWidth;
      const ratio = maxPosition > 0 ? x / maxPosition : 0;
      // 后端拼图块的最大位置是300（320-40-20），最小位置是20
      const actualX = Math.round(20 + ratio * 280); // 20是最小位置，280是可移动范围（300-20）
      console.log('滑块位置:', x, '轨道宽度:', trackWidth, '最大位置:', maxPosition, '比例:', ratio, '实际X坐标:', actualX);

      // 直接返回位置数据，不在滑块验证时调用验证接口
      setIsVerified(true);
      onSuccess({
        id: captchaData.id,
        token: captchaData.token,
        x: actualX,
      });
      // 验证成功后关闭弹窗
      setTimeout(() => {
        onClose();
      }, 500);
    } catch (err) {
      setError(err instanceof Error ? err.message : '验证失败，请重试');
      setTimeout(() => {
        generateCaptcha();
      }, 1000);
    } finally {
      setIsVerifying(false);
    }
  };

  // 处理滑块拖动
  const handleMouseDown = (e: React.MouseEvent) => {
    if (isVerified) return;
    setIsDragging(true);
    startXRef.current = e.clientX;
    currentXRef.current = sliderPosition;
  };

  const handleMouseMove = (e: MouseEvent) => {
    if (!isDragging || !trackRef.current) return;

    const trackWidth = trackRef.current.offsetWidth;
    const sliderWidth = 40; // 滑块宽度
    const deltaX = e.clientX - startXRef.current;
    let newPosition = currentXRef.current + deltaX;

    // 限制滑块位置，考虑滑块宽度
    newPosition = Math.max(0, Math.min(newPosition, trackWidth - sliderWidth));
    
    // 使用 requestAnimationFrame 优化性能
    requestAnimationFrame(() => {
      setSliderPosition(newPosition);
    });
  };

  const handleMouseUp = () => {
    if (!isDragging) return;
    setIsDragging(false);

    // 验证拼图位置
    if (sliderPosition > 0) {
      verifyCaptcha(sliderPosition);
    }
  };

  // 添加全局鼠标事件监听
  useEffect(() => {
    if (isDragging) {
      document.addEventListener('mousemove', handleMouseMove);
      document.addEventListener('mouseup', handleMouseUp);

      return () => {
        document.removeEventListener('mousemove', handleMouseMove);
        document.removeEventListener('mouseup', handleMouseUp);
      };
    }
  }, [isDragging, sliderPosition]);

  // 处理触摸事件
  const handleTouchStart = (e: React.TouchEvent) => {
    if (isVerified) return;
    setIsDragging(true);
    startXRef.current = e.touches[0].clientX;
    currentXRef.current = sliderPosition;
  };

  const handleTouchMove = (e: TouchEvent) => {
    if (!isDragging || !trackRef.current) return;
    
    // 阻止默认行为，防止页面滚动
    e.preventDefault();

    const trackWidth = trackRef.current.offsetWidth;
    const sliderWidth = 40; // 滑块宽度
    const deltaX = e.touches[0].clientX - startXRef.current;
    let newPosition = currentXRef.current + deltaX;

    // 限制滑块位置，考虑滑块宽度
    newPosition = Math.max(0, Math.min(newPosition, trackWidth - sliderWidth));
    
    // 使用 requestAnimationFrame 优化性能
    requestAnimationFrame(() => {
      setSliderPosition(newPosition);
    });
  };

  const handleTouchEnd = () => {
    if (!isDragging) return;
    setIsDragging(false);

    // 验证拼图位置
    if (sliderPosition > 0) {
      verifyCaptcha(sliderPosition);
    }
  };

  // 添加全局触摸事件监听
  useEffect(() => {
    if (isDragging) {
      document.addEventListener('touchmove', handleTouchMove);
      document.addEventListener('touchend', handleTouchEnd);

      return () => {
        document.removeEventListener('touchmove', handleTouchMove);
        document.removeEventListener('touchend', handleTouchEnd);
      };
    }
  }, [isDragging, sliderPosition]);

  // 如果弹窗未打开，不渲染任何内容
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div className="bg-white rounded-lg shadow-lg w-full max-w-md p-6">
        <div className="flex justify-between items-center mb-4">
          <h3 className="text-lg font-medium text-gray-900">安全验证</h3>
          <Button
            variant="ghost"
            size="sm"
            onClick={onClose}
            className="h-8 w-8 p-0"
          >
            <X className="h-4 w-4" />
          </Button>
        </div>

        {error && (
          <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded text-sm text-red-600">
            {error}
          </div>
        )}

        {isVerified && (
          <div className="mb-4 p-3 bg-green-50 border border-green-200 rounded text-sm text-green-600 flex items-center">
            <CheckCircle className="h-4 w-4 mr-2" />
            验证成功
          </div>
        )}

        {captchaData && (
          <div className="space-y-4">
            {/* 拼图区域 */}
            <div className="relative bg-gray-100 rounded overflow-hidden flex justify-center items-center" style={{ height: '160px' }}>
              {/* 背景图片 */}
              <div className="relative" style={{ width: '320px', height: '160px' }}>
                <img
                  src={captchaData.bg_image}
                  alt="验证码背景"
                  className="w-full h-full"
                />
                
                {/* 拼图块 - 使用后端返回的y坐标，并且根据背景图片宽度计算位置 */}
                <div
                  className="absolute pointer-events-none"
                  style={{
                    // 计算拼图块的实际位置，基于后端背景图片宽度320px
                    // 确保拼图块位置与滑块位置匹配
                    // 后端拼图块的最大位置是300（320-40-20），最小位置是20
                    left: `${20 + ((sliderPosition / Math.max(1, (trackRef.current?.offsetWidth || 320) - 40)) * 280)}px`,
                    top: `${captchaData.y}px`,
                  }}
                >
                  <img
                    src={captchaData.puzzle_img}
                    alt="拼图块"
                    className="h-10 w-10"
                    style={{ width: '40px', height: '40px' }}
                  />
                </div>
              </div>
            </div>

            {/* 滑块轨道 */}
            <div
              ref={trackRef}
              className="relative h-10 bg-gray-200 rounded-full overflow-hidden"
            >
              {/* 进度条 */}
              <div
                className="absolute top-0 left-0 h-full bg-blue-500 transition-all duration-100"
                style={{ width: `${sliderPosition}px` }}
              />
              
              {/* 滑块 */}
              <div
                ref={sliderRef}
                className={`absolute top-1/2 w-10 h-10 bg-white rounded-full shadow-md flex items-center justify-center cursor-pointer transform -translate-y-1/2 transition-all ${
                  isDragging ? 'scale-110' : 'hover:scale-105'
                } ${isVerified ? 'bg-green-500' : ''}`}
                style={{ left: `${sliderPosition}px` }}
                onMouseDown={handleMouseDown}
                onTouchStart={handleTouchStart}
              >
                {isVerified ? (
                  <CheckCircle className="h-5 w-5 text-white" />
                ) : (
                  <div className="w-0 h-0 border-t-0 border-b-4 border-l-4 border-r-4 border-b-transparent border-l-gray-600 border-r-gray-600" />
                )}
              </div>
            </div>

            {/* 提示文字 */}
            <div className="text-center text-sm text-gray-600">
              {isVerified ? '验证成功' : '向右拖动滑块完成验证'}
            </div>

            {/* 刷新按钮 */}
            <div className="flex justify-center">
              <Button
                variant="ghost"
                size="sm"
                onClick={generateCaptcha}
                disabled={isVerifying}
                className="text-sm"
              >
                <RefreshCw className={`h-4 w-4 mr-1 ${isVerifying ? 'animate-spin' : ''}`} />
                刷新验证码
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}