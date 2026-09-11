import { SiteFooter, SiteHeader, DocHead } from "../components/SiteLayout";
import { go } from "../lib/go";
import { ArrowLeft, Calendar } from "../components/icons";

/** Generic custom page template (page.html): title plus rendered content. */
export function PageTemplate() {
  return (
    <html lang={go(".Site.Language")}>
      <DocHead
        title={go('printf "%s - %s" .Page.Title .Site.Name')}
        description={go(".Page.Title")}
      />
      <body className="min-h-screen bg-background text-foreground antialiased">
        <SiteHeader />
        <main className="container mx-auto px-4 py-8 max-w-4xl">
          <a
            href="/"
            className="inline-flex items-center gap-2 rounded-md text-sm font-medium transition-colors hover:bg-secondary hover:text-foreground h-9 px-3 mb-6 text-muted-foreground"
          >
            <ArrowLeft className="w-4 h-4" />
            {go('t "back.home"')}
          </a>
          <h1 className="text-3xl md:text-4xl font-bold mb-4">
            {go(".Page.Title")}
          </h1>
          <p className="flex items-center gap-1 text-sm text-muted-foreground mb-8">
            <Calendar className="w-4 h-4" />
            {go('date .Page.UpdatedAt "2006-01-02"')}
          </p>
          <div className="mb-12">
            <div className="prose prose-lg max-w-none">
              {go(".Page.ContentHTML")}
            </div>
          </div>
        </main>
        <SiteFooter />
      </body>
    </html>
  );
}
