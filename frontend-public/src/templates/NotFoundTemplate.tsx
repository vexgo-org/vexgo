import { SiteFooter, SiteHeader, DocHead } from "../components/SiteLayout";
import { go } from "../lib/go";

/** 404 page template, rendered when a public route does not resolve. */
export function NotFoundTemplate() {
  return (
    <html lang={go(".Site.Language")}>
      <DocHead
        title={go('printf (t "notfound.titleFormat") .Site.Name')}
        description=""
      />
      <body className="min-h-screen bg-background text-foreground antialiased">
        <SiteHeader />
        <main className="container mx-auto px-4 py-16 max-w-4xl text-center">
          <h1 className="text-6xl font-bold text-muted-foreground/30 mb-4">
            404
          </h1>
          <p className="text-muted-foreground mb-8">
            {go('t "notfound.desc"')}
          </p>
          <a
            href="/"
            className="inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors bg-primary text-primary-foreground shadow hover:bg-primary/90 h-10 px-4 py-2"
          >
            {go('t "back.home"')}
          </a>
        </main>
        <SiteFooter />
      </body>
    </html>
  );
}
