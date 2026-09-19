import { useEffect, useRef, useState } from "react";
import { ImagePlus, Loader2, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useTranslation } from "@/lib/I18nContext";

export interface ImageDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Prefilled values; set `isEditing` when rewriting an existing image. */
  initial?: { url: string; alt: string } | null;
  isEditing?: boolean;
  onUpload: (file: File) => Promise<string>;
  onSubmit: (values: { url: string; alt: string }) => void;
  onRemove?: () => void;
}

export function ImageDialog({
  open,
  onOpenChange,
  initial,
  isEditing = false,
  onUpload,
  onSubmit,
  onRemove,
}: ImageDialogProps) {
  const { t } = useTranslation();
  const [url, setUrl] = useState("");
  const [alt, setAlt] = useState("");
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState("");
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (!open) return;
    setUrl(initial?.url ?? "");
    setAlt(initial?.alt ?? "");
    setError("");
    setUploading(false);
  }, [open, initial]);

  const handleFile = async (file: File | undefined) => {
    if (!file) return;
    setUploading(true);
    setError("");
    try {
      const uploaded = await onUpload(file);
      if (uploaded) {
        setUrl(uploaded);
        if (!alt) setAlt(file.name.replace(/\.[^.]+$/, ""));
      }
    } catch {
      // `onUpload` already surfaced the failure to the user.
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  };

  const handleSubmit = () => {
    if (!url.trim()) {
      setError(t("editor.imageUrlRequired"));
      return;
    }
    onSubmit({ url: url.trim(), alt: alt.trim() });
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {isEditing ? t("editor.imageEditTitle") : t("editor.imageTitle")}
          </DialogTitle>
        </DialogHeader>

        <div className="grid gap-4">
          <div className="flex items-center gap-3">
            <input
              ref={fileInputRef}
              type="file"
              accept="image/*"
              className="hidden"
              onChange={(event) => void handleFile(event.target.files?.[0])}
            />
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={uploading}
              onClick={() => fileInputRef.current?.click()}
            >
              {uploading ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <ImagePlus className="size-4" />
              )}
              {uploading ? t("common.loading") : t("editor.chooseFile")}
            </Button>
            <span className="text-xs text-muted-foreground">
              {t("editor.uploadHint")}
            </span>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="vexgo-image-url">{t("editor.imageUrl")}</Label>
            <Input
              id="vexgo-image-url"
              value={url}
              placeholder={t("editor.imageUrlPlaceholder")}
              onChange={(event) => setUrl(event.target.value)}
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="vexgo-image-alt">{t("editor.imageAlt")}</Label>
            <Input
              id="vexgo-image-alt"
              value={alt}
              placeholder={t("editor.imageAltPlaceholder")}
              onChange={(event) => setAlt(event.target.value)}
            />
          </div>

          {url.trim() && (
            <div className="flex items-center justify-center rounded-md border bg-muted/30 p-2">
              <img
                src={url.trim()}
                alt={alt}
                className="max-h-40 max-w-full rounded object-contain"
              />
            </div>
          )}

          {error && <p className="text-sm text-destructive">{error}</p>}
        </div>

        <DialogFooter className="sm:justify-between">
          <div>
            {isEditing && onRemove && (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="text-destructive hover:text-destructive"
                onClick={() => {
                  onRemove();
                  onOpenChange(false);
                }}
              >
                <Trash2 className="size-4" />
                {t("editor.removeImage")}
              </Button>
            )}
          </div>
          <div className="flex flex-col-reverse gap-2 sm:flex-row">
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              {t("common.cancel")}
            </Button>
            <Button type="button" onClick={handleSubmit} disabled={uploading}>
              {t("editor.confirm")}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
