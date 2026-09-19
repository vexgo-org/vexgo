import { Facet } from "@codemirror/state";
import type { ImageTarget } from "../types";

/**
 * Callbacks the editor extensions need in order to talk back to React.
 *
 * CodeMirror widgets and DOM handlers are created outside the React tree, so
 * they cannot receive props directly. They read this facet off the editor
 * state instead, which keeps the extensions free of component references and
 * lets a single editor instance be driven by changing props.
 */
export interface EditorCallbacks {
  /** Invoked when the user clicks a rendered image. */
  onImageClick?: (target: ImageTarget) => void;
  /** Invoked when the user picks a language from the in-editor selector. */
  onCodeLanguageChange?: (info: {
    infoFrom: number;
    infoTo: number;
    language: string;
  }) => void;
  /** Uploads an image file and resolves with the URL to insert. */
  uploadImage?: (file: File) => Promise<string>;
  /** Invoked by the link shortcut so React can open its dialog. */
  onRequestLink?: () => void;
  /**
   * Translated strings for the widgets CodeMirror renders itself, which live
   * outside the React tree and therefore cannot call `useTranslation`.
   */
  labels?: {
    codeLanguage: string;
    plainText: string;
  };
}

export const editorCallbacks = Facet.define<EditorCallbacks, EditorCallbacks>({
  combine(values) {
    return values.length > 0 ? Object.assign({}, ...values) : {};
  },
});
