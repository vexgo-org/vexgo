import {
  useCallback,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
} from "react";
import { toast } from "sonner";
import type { EditorView } from "@codemirror/view";

import { getVexGoAPI } from "@/api/generated/endpoints";
import { useIsDark } from "@/hooks/useIsDark";
import { useTranslation } from "@/lib/I18nContext";
import { cn } from "@/lib/utils";

import { insertImage, insertLink, removeImage, replaceImage } from "./commands";
import { ImageDialog } from "./dialogs/ImageDialog";
import { LinkDialog } from "./dialogs/LinkDialog";
import type { EditorCallbacks } from "./extensions/config";
import { EditorToolbar } from "./toolbar/EditorToolbar";
import type { EditorMode, ImageTarget } from "./types";
import { useCodeMirror } from "./useCodeMirror";

export interface MarkdownEditorProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  readOnly?: boolean;
  /** Minimum height of the writing surface, in pixels. */
  minHeight?: number;
  className?: string;
}

export function MarkdownEditor({
  value,
  onChange,
  placeholder,
  readOnly = false,
  minHeight,
  className,
}: MarkdownEditorProps) {
  const { t } = useTranslation();
  const isDark = useIsDark();
  const [mode, setMode] = useState<EditorMode>("rich");
  const viewRef = useRef<EditorView | null>(null);

  const [imageOpen, setImageOpen] = useState(false);
  const [imageTarget, setImageTarget] = useState<ImageTarget | null>(null);
  const [linkOpen, setLinkOpen] = useState(false);
  const [linkSelection, setLinkSelection] = useState("");

  const uploadImage = useCallback(
    async (file: File) => {
      try {
        const response = await getVexGoAPI().postUpload({ file });
        return response.data.file?.url ?? "";
      } catch (error) {
        console.error("Failed to upload image:", error);
        toast.error(t("editor.uploadFailed"));
        throw error;
      }
    },
    [t],
  );

  const openImageDialog = useCallback(() => {
    setImageTarget(null);
    setImageOpen(true);
  }, []);

  const openLinkDialog = useCallback(() => {
    const view = viewRef.current;
    const selection = view?.state.selection.main;
    setLinkSelection(
      view && selection
        ? view.state.sliceDoc(selection.from, selection.to)
        : "",
    );
    setLinkOpen(true);
  }, []);
  const callbacks = useMemo<EditorCallbacks>(
    () => ({
      uploadImage,
      onImageClick: (target) => {
        setImageTarget(target);
        setImageOpen(true);
      },
      onRequestLink: openLinkDialog,
      labels: {
        codeLanguage: t("editor.codeLanguage"),
        plainText: t("editor.plainText"),
      },
    }),
    [uploadImage, openLinkDialog, t],
  );

  const { containerRef, snapshot } = useCodeMirror({
    value,
    onChange,
    placeholder,
    readOnly,
    mode,
    dark: isDark,
    callbacks,
    viewRef,
  });

  const handleImageSubmit = ({ url, alt }: { url: string; alt: string }) => {
    const view = viewRef.current;
    if (!view || view.state.readOnly) return;
    if (imageTarget) {
      replaceImage(view, imageTarget.from, imageTarget.to, url, alt);
    } else {
      insertImage(view, url, alt);
    }
    view.focus();
  };

  const handleImageRemove = () => {
    const view = viewRef.current;
    if (!view || !imageTarget || view.state.readOnly) return;
    removeImage(view, imageTarget.from, imageTarget.to);
    view.focus();
  };

  const handleLinkSubmit = ({ text, url }: { text: string; url: string }) => {
    const view = viewRef.current;
    if (!view || view.state.readOnly) return;
    insertLink(view, url, text);
    view.focus();
  };

  return (
    <div
      className={cn(
        "vexgo-md-editor overflow-hidden rounded-lg border bg-background",
        mode === "rich" ? "vexgo-md-rich" : "vexgo-md-source",
        className,
      )}
      style={
        minHeight
          ? ({
              "--vexgo-md-min-height": `${minHeight}px`,
            } as CSSProperties)
          : undefined
      }
    >
      <EditorToolbar
        viewRef={viewRef}
        snapshot={snapshot}
        mode={mode}
        onModeChange={setMode}
        onRequestImage={openImageDialog}
        onRequestLink={openLinkDialog}
        disabled={readOnly}
      />
      <div ref={containerRef} />

      <ImageDialog
        open={imageOpen}
        onOpenChange={setImageOpen}
        initial={imageTarget}
        isEditing={imageTarget !== null}
        onUpload={uploadImage}
        onSubmit={handleImageSubmit}
        onRemove={imageTarget ? handleImageRemove : undefined}
      />

      <LinkDialog
        open={linkOpen}
        onOpenChange={setLinkOpen}
        initialText={linkSelection}
        onSubmit={handleLinkSubmit}
      />
    </div>
  );
}
