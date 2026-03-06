import React, { useEffect, useRef, useState } from 'react';

type Props = {
  file: File;
  aspect?: number; // width / height
  onCancel: () => void;
  onCrop: (file: File) => void;
  outputWidth?: number;
  outputHeight?: number;
};

export const ImageCropper: React.FC<Props> = ({ file, aspect = 1200 / 630, onCancel, onCrop, outputWidth = 1200, outputHeight = 630 }) => {
  const imgRef = useRef<HTMLImageElement | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const cropBoxRef = useRef<HTMLDivElement | null>(null);
  const [src, setSrc] = useState<string>('');
  const [dragging, setDragging] = useState(false);
  const [pos, setPos] = useState({ x: 0, y: 0 });
  const [start, setStart] = useState<{ x: number; y: number } | null>(null);
  const [scale, setScale] = useState(1);

  useEffect(() => {
    const url = URL.createObjectURL(file);
    setSrc(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);

  const onMouseDown = (e: React.MouseEvent) => {
    e.preventDefault();
    setDragging(true);
    setStart({ x: e.clientX, y: e.clientY });
  };

  const onMouseMove = (e: React.MouseEvent) => {
    if (!dragging || !start) return;
    const dx = e.clientX - start.x;
    const dy = e.clientY - start.y;
    setStart({ x: e.clientX, y: e.clientY });
    setPos((p) => ({ x: p.x + dx, y: p.y + dy }));
  };

  const onMouseUp = () => {
    setDragging(false);
    setStart(null);
  };

  const handleWheel = (e: React.WheelEvent) => {
    e.preventDefault();
    const delta = -e.deltaY / 500;
    setScale((s) => Math.max(0.2, Math.min(5, +((s + delta).toFixed(2)))));
  };

  const doCrop = async () => {
    if (!imgRef.current || !containerRef.current) return;
    const img = imgRef.current;

    const imgRect = img.getBoundingClientRect();
    const cropRect = cropBoxRef.current?.getBoundingClientRect();
    if (!cropRect) return;

    // Displayed image size (including transforms)
    const dispW = imgRect.width;
    const dispH = imgRect.height;

    // Top-left of crop box relative to image (imgRect already includes transforms)
    const cropLeft = cropRect.left - imgRect.left;
    const cropTop = cropRect.top - imgRect.top;

    // Map to original image natural size
    const ratioX = img.naturalWidth / dispW;
    const ratioY = img.naturalHeight / dispH;

    let sx = Math.round(cropLeft * ratioX);
    let sy = Math.round(cropTop * ratioY);
    let sw = Math.round(cropRect.width * ratioX);
    let sh = Math.round(cropRect.height * ratioY);

    // Clamp to image bounds to avoid drawing outside source
    if (sx < 0) sx = 0;
    if (sy < 0) sy = 0;
    if (sx + sw > img.naturalWidth) sw = img.naturalWidth - sx;
    if (sy + sh > img.naturalHeight) sh = img.naturalHeight - sy;
    if (sw <= 0 || sh <= 0) return;

    // Create canvas with target output size so uploaded image has consistent dimensions
    const canvas = document.createElement('canvas');
    canvas.width = outputWidth;
    canvas.height = outputHeight;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const image = new Image();
    image.crossOrigin = 'anonymous';
    image.src = src;
    await new Promise((res, rej) => {
      image.onload = res;
      image.onerror = rej;
    });

    // Draw and scale the selected source region to the desired output size
    ctx.drawImage(image, sx, sy, sw, sh, 0, 0, outputWidth, outputHeight);

    return new Promise<void>((resolve, reject) => {
      const mime = file.type || 'image/jpeg';
      // For lossy formats (jpeg, webp) pass quality=1 to avoid additional compression
      const isLossy = /(jpeg|jpg|webp)/i.test(mime);
      const quality = isLossy ? 1.0 : undefined;
      canvas.toBlob((blob) => {
        if (!blob) {
          reject(new Error('裁剪失败'));
          return;
        }
        const ext = /png/i.test(mime) ? 'png' : /webp/i.test(mime) ? 'webp' : 'jpg';
        const newFile = new File([blob], file.name.replace(/\.[^.]+$/, '' ) + '-cover.' + ext, { type: blob.type });
        onCrop(newFile);
        resolve();
      }, mime, quality as any);
    });
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-white rounded shadow-lg p-4 max-w-4xl w-full mx-4">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-lg font-medium">裁剪封面（保持比例）</h3>
          <div className="flex gap-2">
            <button className="btn" onClick={onCancel}>取消</button>
            <button className="btn btn-primary" onClick={() => doCrop()}>确定并上传</button>
          </div>
        </div>

        <div className="w-full overflow-hidden touch-none" onWheel={handleWheel}>
          <div
            ref={containerRef}
            style={{
              width: '100%',
              // crop box height based on aspect
              height: `calc(100% )`,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              position: 'relative',
              minHeight: 360
            }}
          >
            <div
              onMouseDown={onMouseDown}
              onMouseMove={onMouseMove}
              onMouseUp={onMouseUp}
              onMouseLeave={onMouseUp}
              style={{
                cursor: dragging ? 'grabbing' : 'grab',
                overflow: 'hidden',
                width: '100%',
                height: '100%',
                position: 'relative'
              }}
            >
              <img
                ref={imgRef}
                src={src}
                alt="to-crop"
                style={{
                  transform: `translate(${pos.x}px, ${pos.y}px) scale(${scale})`,
                  transformOrigin: 'center center',
                  userSelect: 'none',
                  display: 'block',
                  maxWidth: 'none',
                }}
                draggable={false}
                onLoad={(e) => {
                  const imgEl = e.currentTarget;
                  // center image
                  setPos({ x: 0, y: 0 });
                  // fit image to container width
                  const container = containerRef.current;
                  if (container) {
                    const contW = container.clientWidth;
                    const ratio = contW / imgEl.naturalWidth;
                    imgEl.style.width = `${Math.round(imgEl.naturalWidth * ratio)}px`;
                    imgEl.style.height = 'auto';
                  }
                }}
              />

              {/* crop box overlay */}
              <div
                style={{
                  position: 'absolute',
                  left: '50%',
                  top: '50%',
                  transform: 'translate(-50%, -50%)',
                  width: '90%',
                  maxWidth: 1000,
                  aspectRatio: `${aspect}`,
                  border: '2px dashed rgba(255,255,255,0.9)',
                  boxShadow: '0 0 0 9999px rgba(0,0,0,0.4)'
                }}
                ref={cropBoxRef}
              />
            </div>
          </div>

          <div className="mt-3 flex items-center gap-3">
            <label className="text-sm">缩放：</label>
            <input type="range" min={0.2} max={5} step={0.01} value={scale} onChange={(e) => setScale(Number(e.target.value))} />
            <span className="text-xs text-gray-500">拖动图片以移动，滚轮缩放</span>
          </div>
        </div>
      </div>
    </div>
  );
};

export default ImageCropper;
