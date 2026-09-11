import { SiteFooter, SiteHeader, DocHead } from "../components/SiteLayout";
import { go } from "../lib/go";
import { ArrowLeft } from "../components/icons";

/**
 * Links page template (links.html). Friend cards are expanded server-side
 * from the fenced friends block into .vexgo-friends HTML; themes only style
 * the card CSS. The page content renders above the cards as usual.
 */
export function LinksTemplate() {
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
          <h1 className="text-3xl md:text-4xl font-bold mb-8">
            {go(".Page.Title")}
          </h1>
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
