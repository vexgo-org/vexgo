import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import App from "./App.tsx";
import { applyTheme, readStoredTheme } from "@/hooks/useTheme";
import { loadLocale, resolveInitialLocale, setLocale } from "@/lib/i18n";

// Resolve the theme before the first paint: doing it in an effect would show
// a flash of the wrong mode on every cold load.
applyTheme(readStoredTheme());

// Message packs are code-split, so the active one has to be in memory before
// React renders — otherwise every label would paint as its raw key. Only the
// active pack is fetched.
async function bootstrap() {
  const locale = resolveInitialLocale();
  try {
    await loadLocale(locale);
  } catch (error) {
    // Render anyway: raw keys beat a blank page if the chunk never arrives.
    console.error(`Failed to load the ${locale} message pack:`, error);
  }
  setLocale(locale);

  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
}

void bootstrap();
