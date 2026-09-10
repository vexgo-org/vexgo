import { go } from "../lib/go";

/** SiteHeader is the top navigation bar shared by every public page. */
export function SiteHeader() {
  return (
    <header className="border-b">
      <div className="container mx-auto px-4 h-16 flex items-center justify-between">
        <a
          href="/"
          className="text-xl font-bold hover:text-primary transition-colors"
        >
          {go(".Site.Name")}
        </a>
        <nav className="flex items-center gap-4 text-sm">
          <a
            href="/"
            className="text-muted-foreground hover:text-foreground transition-colors"
          >
            Home
          </a>
        </nav>
      </div>
    </header>
  );
}

/** SiteFooter is the footer shared by every public page. */
export function SiteFooter() {
  return (
    <footer className="border-t py-6 text-center text-sm text-muted-foreground">
      <a href="/" className="hover:text-foreground transition-colors">
        {go(".Site.Name")}
      </a>{" "}
      · Powered by VexGo
    </footer>
  );
}

/** DocHead renders the shared <head> metadata with per-page title/description. */
export function DocHead({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <head>
      <meta charSet="utf-8" />
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <title>{title}</title>
      <meta name="description" content={description} />
      <link rel="stylesheet" href="/theme-assets/style.css" />
    </head>
  );
}
