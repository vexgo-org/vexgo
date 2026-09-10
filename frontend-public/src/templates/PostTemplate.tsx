import { SiteFooter, SiteHeader, DocHead } from "../components/SiteChrome";
import { go } from "../lib/go";
import {
  ArrowLeft,
  Calendar,
  Clock,
  Eye,
  Heart,
  MessageCircle,
  Share2,
} from "../components/icons";

/**
 * Post detail page template ({{.Post.ContentHTML}} is pre-rendered markdown).
 * The like and share controls are wired by the comment widget
 * (widget/comments.js) via their data attributes; without JavaScript they
 * degrade to static counts.
 */
export function PostTemplate() {
  return (
    <html lang="en">
      <DocHead
        title={go('printf "%s - %s" .Post.Title .Site.Name')}
        description={go(".Post.Excerpt")}
      />
      <body className="min-h-screen bg-background text-foreground antialiased">
        <SiteHeader />
        <main className="container mx-auto px-4 py-8 max-w-4xl">
          {/* Back button */}
          <a
            href="/"
            className="inline-flex items-center gap-2 rounded-md text-sm font-medium transition-colors hover:bg-secondary hover:text-foreground h-9 px-3 mb-6 text-muted-foreground"
          >
            <ArrowLeft className="w-4 h-4" />
            Back to home
          </a>

          {/* Post header */}
          <div className="mb-8">
            {/* Category and tags */}
            <div className="flex flex-wrap items-center gap-2 mb-4">
              {go("if .Post.Category")}
              <span className="inline-flex items-center rounded-md border border-transparent bg-secondary px-2.5 py-0.5 text-xs font-semibold text-secondary-foreground">
                {go(".Post.Category")}
              </span>
              {go("end")}
              {go("range .Post.Tags")}
              <span className="inline-flex items-center rounded-md border px-2.5 py-0.5 text-xs font-semibold">
                {go(".")}
              </span>
              {go("end")}
            </div>

            {/* Title */}
            <h1 className="text-3xl md:text-4xl font-bold mb-6">
              {go(".Post.Title")}
            </h1>

            {/* Author info */}
            <div className="flex items-center flex-wrap gap-4">
              <a
                href={go("userURL .Post.AuthorID")}
                className="flex items-center gap-4"
              >
                {go("if .Post.AuthorAvatar")}
                <img
                  src={go(".Post.AuthorAvatar")}
                  alt="Avatar"
                  className="w-12 h-12 rounded-full object-cover"
                />
                {go("else")}
                <span className="w-12 h-12 rounded-full bg-primary/10 text-primary text-lg font-medium flex items-center justify-center">
                  {go('printf "%.1s" .Post.AuthorName')}
                </span>
                {go("end")}
                <div>
                  <p className="font-medium hover:text-primary transition-colors">
                    {go(".Post.AuthorName")}
                  </p>
                  <div className="flex items-center gap-3 text-sm text-muted-foreground">
                    <span className="flex items-center gap-1">
                      <Calendar className="w-4 h-4" />
                      {go('date .Post.CreatedAt "2006-01-02"')}
                    </span>
                    {go("if ne .Post.UpdatedAt .Post.CreatedAt")}
                    <span className="flex items-center gap-1">
                      <Clock className="w-4 h-4" />
                      Updated {go('date .Post.UpdatedAt "2006-01-02"')}
                    </span>
                    {go("end")}
                  </div>
                </div>
              </a>
            </div>
          </div>

          {/* Cover image */}
          {go("if .Post.CoverImage")}
          <div className="mb-8 rounded-lg overflow-hidden">
            <img
              src={go(".Post.CoverImage")}
              alt={go(".Post.Title")}
              className="w-full max-h-[400px] object-fill"
            />
          </div>
          {go("end")}

          {/* Post content */}
          <div className="mb-12">
            <div className="prose prose-lg max-w-none">
              {go(".Post.ContentHTML")}
            </div>
          </div>

          {/* Interaction area */}
          <div className="flex items-center justify-between py-6 border-y">
            <div className="flex items-center gap-4">
              <button
                type="button"
                data-like-post-id={go(".Post.ID")}
                className="like-btn inline-flex items-center justify-center gap-2 rounded-md border border-input bg-background px-4 h-10 text-sm font-medium shadow-sm transition-colors hover:bg-secondary"
              >
                <Heart className="w-5 h-5" />
                <span data-like-count>{go(".Post.LikesCount")}</span>
              </button>
              <a
                href="#vexgo-comments"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-input bg-background px-4 h-10 text-sm font-medium shadow-sm transition-colors hover:bg-secondary"
              >
                <MessageCircle className="w-5 h-5" />
                <span>Comments ({go(".Post.CommentsCount")})</span>
              </a>
              <span className="flex items-center gap-1 text-sm text-muted-foreground">
                <Eye className="w-5 h-5" />
                <span>{go(".Post.ViewCount")}</span>
              </span>
            </div>
            <div className="relative">
              <button
                type="button"
                data-share-url={go(".Post.URL")}
                className="inline-flex items-center justify-center rounded-md text-sm font-medium transition-colors hover:bg-secondary h-9 w-9"
                aria-label="Share"
              >
                <Share2 className="w-5 h-5" />
              </button>
              <span
                data-share-hint
                hidden
                className="absolute -bottom-6 left-1/2 -translate-x-1/2 text-[10px] text-muted-foreground bg-muted px-2 py-1 rounded whitespace-nowrap"
              >
                Link copied
              </span>
            </div>
          </div>

          {/* Comment section: rendered by the built-in widget (see
              widget/comments.js). The widget reads the post id from this
              container and styles itself with inline styles, so any theme
              can adopt it with the same two lines. */}
          <div className="mt-12">
            <div id="vexgo-comments" data-post-id={go(".Post.ID")}></div>
          </div>
        </main>
        <SiteFooter /> <script src="/theme-assets/comments.js" defer></script>
      </body>
    </html>
  );
}
