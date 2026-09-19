import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import App from "./App.tsx";
import { applyTheme, readStoredTheme } from "@/hooks/useTheme";

// Resolve the theme before the first paint: doing it in an effect would show
// a flash of the wrong mode on every cold load.
applyTheme(readStoredTheme());

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
