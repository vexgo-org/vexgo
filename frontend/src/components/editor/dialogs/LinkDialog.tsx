import { useEffect, useState } from "react";

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

export interface LinkDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Text of the current selection, used as the link label. */
  initialText?: string;
  initialUrl?: string;
  onSubmit: (values: { text: string; url: string }) => void;
}

export function LinkDialog({
  open,
  onOpenChange,
  initialText = "",
  initialUrl = "",
  onSubmit,
}: LinkDialogProps) {
  const { t } = useTranslation();
  const [text, setText] = useState(initialText);
  const [url, setUrl] = useState(initialUrl);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!open) return;
    setText(initialText);
    setUrl(initialUrl);
    setError("");
  }, [open, initialText, initialUrl]);

  const handleSubmit = () => {
    if (!url.trim()) {
      setError(t("editor.linkUrlRequired"));
      return;
    }
    onSubmit({ text: text.trim(), url: url.trim() });
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("editor.linkTitle")}</DialogTitle>
        </DialogHeader>

        <div className="grid gap-4">
          <div className="grid gap-2">
            <Label htmlFor="vexgo-link-text">{t("editor.linkText")}</Label>
            <Input
              id="vexgo-link-text"
              value={text}
              placeholder={t("editor.linkTextPlaceholder")}
              onChange={(event) => setText(event.target.value)}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="vexgo-link-url">{t("editor.linkUrl")}</Label>
            <Input
              id="vexgo-link-url"
              value={url}
              placeholder={t("editor.linkUrlPlaceholder")}
              onChange={(event) => setUrl(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  handleSubmit();
                }
              }}
            />
          </div>
          {error && <p className="text-sm text-destructive">{error}</p>}
        </div>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            {t("common.cancel")}
          </Button>
          <Button type="button" onClick={handleSubmit}>
            {t("editor.confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
