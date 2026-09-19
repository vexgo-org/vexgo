/** The two editing modes offered by {@link MarkdownEditor}. */
export type EditorMode = "rich" | "source";

/** Block type reported by the toolbar's block-type selector. */
export type BlockType =
  | "paragraph"
  | "heading1"
  | "heading2"
  | "heading3"
  | "heading4"
  | "quote"
  | "code";

/**
 * A flattened snapshot of everything the toolbar needs to render itself.
 * Rebuilt from the CodeMirror state whenever the document or selection
 * changes, so the toolbar never reaches into the editor directly.
 */
export interface EditorSnapshot {
  bold: boolean;
  italic: boolean;
  strike: boolean;
  inlineCode: boolean;
  blockType: BlockType;
  inBulletList: boolean;
  inOrderedList: boolean;
  inTaskList: boolean;
  inQuote: boolean;
  inCodeBlock: boolean;
  /** Info string of the enclosing fenced code block, "" when absent. */
  codeLanguage: string;
  /** Range of the enclosing code block's info string, null when absent. */
  codeInfoRange: { from: number; to: number } | null;
  canUndo: boolean;
  canRedo: boolean;
}

/** Payload handed to the image dialog when editing an existing image. */
export interface ImageTarget {
  /** Document range covering the whole `![alt](url)` construct. */
  from: number;
  to: number;
  url: string;
  alt: string;
}

/** Payload handed to the link dialog when editing an existing link. */
export interface LinkTarget {
  /** Document range covering the link's text (not the brackets). */
  from: number;
  to: number;
  url: string;
  text: string;
}
