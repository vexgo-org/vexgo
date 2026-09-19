import type { RefObject } from "react";
import type { EditorView } from "@codemirror/view";
import { redo, undo } from "@codemirror/commands";
import {
  Bold,
  Code,
  ImagePlus,
  Italic,
  Link2,
  List,
  ListChecks,
  ListOrdered,
  Minus,
  Redo2,
  Strikethrough,
  Table2,
  Undo2,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useTranslation } from "@/lib/I18nContext";
import { cn } from "@/lib/utils";

import { CODE_LANGUAGES } from "../codeLanguages";
import {
  insertCodeBlock,
  insertHorizontalRule,
  insertTable,
  setCodeLanguage,
  setHeading,
  setParagraph,
  toggleBold,
  toggleBulletList,
  toggleInlineCode,
  toggleItalic,
  toggleOrderedList,
  toggleQuote,
  toggleStrike,
  toggleTaskList,
} from "../commands";
import type { BlockType, EditorMode, EditorSnapshot } from "../types";
import { ToolbarButton, ToolbarDivider } from "./ToolbarButton";

export interface EditorToolbarProps {
  viewRef: RefObject<EditorView | null>;
  snapshot: EditorSnapshot;
  mode: EditorMode;
  onModeChange: (mode: EditorMode) => void;
  onRequestImage: () => void;
  onRequestLink: () => void;
  disabled?: boolean;
}

export function EditorToolbar({
  viewRef,
  snapshot,
  mode,
  onModeChange,
  onRequestImage,
  onRequestLink,
  disabled = false,
}: EditorToolbarProps) {
  const { t } = useTranslation();

  const run = (command: (view: EditorView) => boolean) => {
    const view = viewRef.current;
    if (!view) return;
    command(view);
    view.focus();
  };

  const applyBlock = (value: string) => {
    const view = viewRef.current;
    if (!view) return;
    switch (value as BlockType) {
      case "paragraph":
        setParagraph(view);
        break;
      case "heading1":
        setHeading(view, 1);
        break;
      case "heading2":
        setHeading(view, 2);
        break;
      case "heading3":
        setHeading(view, 3);
        break;
      case "heading4":
        setHeading(view, 4);
        break;
      case "quote":
        if (!snapshot.inQuote) toggleQuote(view);
        break;
      case "code":
        if (!snapshot.inCodeBlock) insertCodeBlock(view);
        break;
      default:
        break;
    }
    view.focus();
  };

  const off = disabled;

  return (
    <div className="flex flex-wrap items-center gap-1 border-b bg-muted/50 p-2">
      <ToolbarButton
        label={t("editor.undo")}
        disabled={off || !snapshot.canUndo}
        onClick={() => run(undo)}
      >
        <Undo2 />
      </ToolbarButton>
      <ToolbarButton
        label={t("editor.redo")}
        disabled={off || !snapshot.canRedo}
        onClick={() => run(redo)}
      >
        <Redo2 />
      </ToolbarButton>

      <ToolbarDivider />

      <Select value={snapshot.blockType} onValueChange={applyBlock}>
        <SelectTrigger
          size="sm"
          className="w-[110px]"
          title={t("editor.blockType")}
          disabled={off}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="paragraph">{t("editor.paragraph")}</SelectItem>
          <SelectItem value="heading1">{t("editor.heading1")}</SelectItem>
          <SelectItem value="heading2">{t("editor.heading2")}</SelectItem>
          <SelectItem value="heading3">{t("editor.heading3")}</SelectItem>
          <SelectItem value="heading4">{t("editor.heading4")}</SelectItem>
          <SelectItem value="quote">{t("editor.quote")}</SelectItem>
          <SelectItem value="code">{t("editor.codeBlock")}</SelectItem>
        </SelectContent>
      </Select>

      <ToolbarDivider />

      <ToolbarButton
        label={t("editor.bold")}
        active={snapshot.bold}
        disabled={off}
        onClick={() => run(toggleBold)}
      >
        <Bold />
      </ToolbarButton>
      <ToolbarButton
        label={t("editor.italic")}
        active={snapshot.italic}
        disabled={off}
        onClick={() => run(toggleItalic)}
      >
        <Italic />
      </ToolbarButton>
      <ToolbarButton
        label={t("editor.strike")}
        active={snapshot.strike}
        disabled={off}
        onClick={() => run(toggleStrike)}
      >
        <Strikethrough />
      </ToolbarButton>
      <ToolbarButton
        label={t("editor.inlineCode")}
        active={snapshot.inlineCode}
        disabled={off}
        onClick={() => run(toggleInlineCode)}
      >
        <Code />
      </ToolbarButton>

      <ToolbarDivider />

      <ToolbarButton
        label={t("editor.bulletList")}
        active={snapshot.inBulletList}
        disabled={off}
        onClick={() => run(toggleBulletList)}
      >
        <List />
      </ToolbarButton>
      <ToolbarButton
        label={t("editor.orderedList")}
        active={snapshot.inOrderedList}
        disabled={off}
        onClick={() => run(toggleOrderedList)}
      >
        <ListOrdered />
      </ToolbarButton>
      <ToolbarButton
        label={t("editor.taskList")}
        active={snapshot.inTaskList}
        disabled={off}
        onClick={() => run(toggleTaskList)}
      >
        <ListChecks />
      </ToolbarButton>

      <ToolbarDivider />

      <ToolbarButton
        label={t("editor.link")}
        disabled={off}
        onClick={onRequestLink}
      >
        <Link2 />
      </ToolbarButton>
      <ToolbarButton
        label={t("editor.image")}
        disabled={off}
        onClick={onRequestImage}
      >
        <ImagePlus />
      </ToolbarButton>
      <ToolbarButton
        label={t("editor.hr")}
        disabled={off}
        onClick={() => run(insertHorizontalRule)}
      >
        <Minus />
      </ToolbarButton>
      <ToolbarButton
        label={t("editor.table")}
        disabled={off}
        onClick={() =>
          run((view) => insertTable(view, t("editor.tableHeader")))
        }
      >
        <Table2 />
      </ToolbarButton>

      {snapshot.inCodeBlock && snapshot.codeInfoRange && (
        <>
          <ToolbarDivider />
          <Select
            value={snapshot.codeLanguage || "__plain__"}
            onValueChange={(value) => {
              const view = viewRef.current;
              if (!view || !snapshot.codeInfoRange) return;
              setCodeLanguage(
                view,
                snapshot.codeInfoRange.from,
                snapshot.codeInfoRange.to,
                value === "__plain__" ? "" : value,
              );
              view.focus();
            }}
          >
            <SelectTrigger
              size="sm"
              className="w-[150px]"
              title={t("editor.codeLanguage")}
              disabled={off}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__plain__">{t("editor.plainText")}</SelectItem>
              {CODE_LANGUAGES.map((item) => (
                <SelectItem key={item.value} value={item.value}>
                  {item.label}
                </SelectItem>
              ))}
              {snapshot.codeLanguage &&
                !CODE_LANGUAGES.some(
                  (item) => item.value === snapshot.codeLanguage,
                ) && (
                  <SelectItem value={snapshot.codeLanguage}>
                    {snapshot.codeLanguage}
                  </SelectItem>
                )}
            </SelectContent>
          </Select>
        </>
      )}

      <div className="ml-auto flex items-center gap-1 rounded-md bg-background p-0.5">
        {(["rich", "source"] as const).map((value) => (
          <Button
            key={value}
            type="button"
            variant="ghost"
            size="sm"
            className={cn(
              "h-7 px-2 text-xs text-muted-foreground",
              mode === value && "bg-accent text-accent-foreground",
            )}
            aria-pressed={mode === value}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => onModeChange(value)}
          >
            {value === "rich" ? t("editor.modeRich") : t("editor.modeSource")}
          </Button>
        ))}
      </div>
    </div>
  );
}
