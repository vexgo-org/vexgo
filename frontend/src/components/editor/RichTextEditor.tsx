import { useRef, useCallback, useEffect, useState, forwardRef, useImperativeHandle } from "react";
import { useIsDark } from "@/hooks/useIsDark";
import { uploadApi } from "@/lib/api";
import {
  MDXEditor,
  type MDXEditorMethods,
  toolbarPlugin,
  UndoRedo,
  BoldItalicUnderlineToggles,
  BlockTypeSelect,
  CreateLink,
  InsertImage,
  ListsToggle,
  CodeToggle,
  InsertThematicBreak,
  InsertTable,
  InsertCodeBlock,
  headingsPlugin,
  listsPlugin,
  linkPlugin,
  linkDialogPlugin,
  imagePlugin,
  thematicBreakPlugin,
  codeBlockPlugin,
  codeMirrorPlugin,
  directivesPlugin,
  frontmatterPlugin,
  AdmonitionDirectiveDescriptor,
  quotePlugin,
  markdownShortcutPlugin,
  diffSourcePlugin,
  DiffSourceToggleWrapper,
  ConditionalContents,
  ChangeCodeMirrorLanguage,
} from "@mdxeditor/editor";
import "@mdxeditor/editor/style.css";

export interface RichTextEditorRef {
  uploadTempImages: () => Promise<Array<{ tempUrl: string; realUrl: string }>>;
}

interface RichTextEditorProps {
  content: string;
  onChange: (content: string) => void;
  placeholder?: string;
  originalContent?: string;
  onImagesUploaded?: (images: Array<{ tempUrl: string; realUrl: string }>) => void;
}

interface TempImage {
  file: File;
  tempUrl: string;
}

export const RichTextEditor = forwardRef<RichTextEditorRef, RichTextEditorProps>(({
  content,
  onChange,
  placeholder,
  originalContent,
  onImagesUploaded,
}, ref) => {
  const editorRef = useRef<MDXEditorMethods>(null);
  const isDark = useIsDark();
  const isInternalChange = useRef(false);
  const [tempImages, setTempImages] = useState<TempImage[]>([]);

  // Clean up temporary URLs when component unmounts
  useEffect(() => {
    return () => {
      tempImages.forEach(img => URL.revokeObjectURL(img.tempUrl));
    };
  }, [tempImages]);

  // Image upload handler for temporary storage
  const imageUploadHandler = useCallback(
    async (file: File): Promise<string> => {
      // Create temporary URL for preview
      const tempUrl = URL.createObjectURL(file);
      
      // Store the temporary image
      setTempImages(prev => [...prev, { file, tempUrl }]);
      
      return tempUrl;
    },
    [],
  );

  // Upload all temporary images to server
  const uploadTempImages = useCallback(async (): Promise<Array<{ tempUrl: string; realUrl: string }>> => {
    if (tempImages.length === 0) return [];

    const uploadPromises = tempImages.map(async (tempImg) => {
      try {
        const response = await uploadApi.uploadFile(tempImg.file);
        const realUrl = response.data.file!.url;
        return { tempUrl: tempImg.tempUrl, realUrl };
      } catch (error) {
        console.error("Failed to upload image:", error);
        throw error;
      }
    });

    const results = await Promise.all(uploadPromises);
    
    // Revoke temporary URLs
    tempImages.forEach(img => URL.revokeObjectURL(img.tempUrl));
    setTempImages([]);

    if (onImagesUploaded) {
      onImagesUploaded(results);
    }

    return results;
  }, [tempImages, onImagesUploaded]);

  // Expose uploadTempImages method through ref
  useImperativeHandle(ref, () => ({
    uploadTempImages,
  }));

  // Handle content changes
  const handleChange = useCallback(
    (markdown: string) => {
      isInternalChange.current = true;
      onChange(markdown);
    },
    [onChange],
  );

  // Use useEffect to sync content to editor
  // Only call setMarkdown when content is updated from external source (not from editor itself)
  // to avoid resetting cursor position on every input in source mode
  useEffect(() => {
    if (isInternalChange.current) {
      isInternalChange.current = false;
      return;
    }
    if (editorRef.current && content !== undefined) {
      editorRef.current.setMarkdown(content || "");
    }
  }, [content]);

  const defaultCodeBlockLanguage = "js";
  const codeBlockLanguages = {
    js: "JavaScript",
    jsx: "JavaScript (React)",
    ts: "TypeScript",
    tsx: "TypeScript (React)",
    css: "CSS",
    html: "HTML",
    python: "Python",
    bash: "Bash",
    shell: "Shell",
    json: "JSON",
    yaml: "YAML",
    markdown: "Markdown",
  };

  return (
    <div className="border rounded-lg overflow-hidden bg-background">
      <MDXEditor
        ref={editorRef}
        markdown={content || ""}
        onChange={handleChange}
        placeholder={placeholder || "Start writing..."}
        contentEditableClassName="prose"
        className={isDark ? "dark-theme dark-editor" : "dark-editor"}
        plugins={[
          headingsPlugin(),
          listsPlugin(),
          linkPlugin(),
          linkDialogPlugin(),
          imagePlugin({
            imageUploadHandler,
          }),
          thematicBreakPlugin(),
          codeBlockPlugin({ defaultCodeBlockLanguage }),
          codeMirrorPlugin({ codeBlockLanguages }),
          directivesPlugin({
            directiveDescriptors: [AdmonitionDirectiveDescriptor],
          }),
          frontmatterPlugin(),
          quotePlugin(),
          markdownShortcutPlugin(),
          diffSourcePlugin({
            viewMode: "rich-text",
            diffMarkdown: originalContent ?? "",
            readOnlyDiff: false,
          }),
          toolbarPlugin({
            toolbarClassName:
              "flex flex-wrap items-center gap-1 p-2 border-b bg-muted/50",
            toolbarContents: () => (
              <DiffSourceToggleWrapper>
                <ConditionalContents
                  options={[
                    {
                      when: (editor) => editor?.editorType === "codeblock",
                      contents: () => <ChangeCodeMirrorLanguage />,
                    },
                    {
                      fallback: () => (
                        <>
                          <UndoRedo />
                          <span className="w-px h-6 bg-border mx-1" />
                          <BoldItalicUnderlineToggles />
                          <CodeToggle />
                          <span className="w-px h-6 bg-border mx-1" />
                          <BlockTypeSelect />
                          <span className="w-px h-6 bg-border mx-1" />
                          <ListsToggle />
                          <span className="w-px h-6 bg-border mx-1" />
                          <CreateLink />
                          <InsertImage />
                          <InsertTable />
                          <InsertCodeBlock />
                          <InsertThematicBreak />
                        </>
                      ),
                    },
                  ]}
                />
              </DiffSourceToggleWrapper>
            ),
          }),
        ]}
      />
    </div>
  );
});

RichTextEditor.displayName = "RichTextEditor";