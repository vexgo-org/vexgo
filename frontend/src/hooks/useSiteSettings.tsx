import { useCallback, useEffect, useState } from "react";

import { getVexGoAPI } from "@/api/generated/endpoints";

/**
 * Site name and icon for the console chrome. Both shells need them, and the
 * document title / favicon belong to whichever shell is mounted, so the
 * fetch lives here rather than being repeated per layout.
 */
export function useSiteSettings() {
  const [siteName, setSiteName] = useState("VexGo");
  const [siteIcon, setSiteIcon] = useState("");

  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      try {
        const response = await getVexGoAPI().getConfigGeneral();
        if (cancelled) return;

        if (response.data.siteName) {
          setSiteName(response.data.siteName);
          document.title = response.data.siteName;
        }

        if (response.data.siteIcon) {
          setSiteIcon(response.data.siteIcon);
          let link =
            document.querySelector<HTMLLinkElement>("link[rel~='icon']");
          if (!link) {
            link = document.createElement("link");
            link.rel = "icon";
            document.head.appendChild(link);
          }
          // The bundled mark ships as scheme-scoped links (the light one is
          // the server's /favicon.ico). A configured icon replaces both, so
          // clear the media query that would otherwise keep it out of one
          // color scheme.
          link.media = "";
          link.href = response.data.siteIcon;
        }
      } catch {
        // Non-fatal: the console still works with the bundled name and icon.
      }
    };

    load();
    return () => {
      cancelled = true;
    };
  }, []);

  return { siteName, siteIcon };
}

/**
 * The brand mark: the configured site icon, or the bundled VexGo glyph that
 * swaps with the theme.
 */
export function useSiteBrand() {
  const { siteName, siteIcon } = useSiteSettings();
  const render = useCallback(
    (className: string) =>
      siteIcon ? (
        <img src={siteIcon} alt="" className={className} />
      ) : (
        <>
          <img
            src="/admin/assets/vexgo-light.ico"
            alt=""
            className={`${className} dark:hidden`}
          />
          <img
            src="/admin/assets/vexgo-dark.ico"
            alt=""
            className={`${className} hidden dark:block`}
          />
        </>
      ),
    [siteIcon],
  );

  return { siteName, siteIcon, renderBrand: render };
}
