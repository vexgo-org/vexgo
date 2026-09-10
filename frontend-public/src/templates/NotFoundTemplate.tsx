import { SiteFooter, SiteHeader, DocHead } from "../components/SiteChrome";
import { go } from "../lib/go";

/** 404 page template, rendered when a public route does not resolve. */
export function NotFoundTemplate() {
  return (
    <html lang="en">
      <DocHead
        title={go('printf "Not Found - %s" .Site.Name')}
        description=""
      />
      <body className="min-h-screen bg-background text-foreground antialiased">
        <SiteHeader />
        <main className="container mx-auto px-4 py-24 text-center">
          <h1 className="text-4xl font-bold mb-4">404</h1>
          <p className="text-muted-foreground mb-6">Page not found.</p>
          <a
            href="/"
            className="text-primary underline hover:opacity-80 transition-opacity"
          >
            Back to home
          </a>
        </main>
        <SiteFooter />
      </body>
    </html>
  );
}
