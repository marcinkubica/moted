import { useEffect, useRef, useState } from "react";

type Theme =
  | "light"
  | "dark"
  | "dimmed"
  | "high-contrast"
  | "catppuccin"
  | "tokyo-night"
  | "tokyo-night-light"
  | "solarized-light";

const THEMES: { id: Theme; name: string; swatches: [string, string, string] }[] = [
  { id: "light", name: "Light", swatches: ["#ffffff", "#f0f2f5", "#1f2328"] },
  { id: "dark", name: "Dark", swatches: ["#0d1117", "#161b22", "#e6edf3"] },
  { id: "dimmed", name: "Dimmed", swatches: ["#22272e", "#2d333b", "#adbac7"] },
  { id: "high-contrast", name: "High Contrast", swatches: ["#010409", "#0d1117", "#f0f6fc"] },
  { id: "catppuccin", name: "Catppuccin", swatches: ["#1e1e2e", "#181825", "#cdd6f4"] },
  { id: "tokyo-night", name: "Tokyo Night", swatches: ["#1a1b26", "#16161e", "#c0caf5"] },
  { id: "tokyo-night-light", name: "Tokyo Night Light", swatches: ["#d5d6db", "#cbccd1", "#343b58"] },
  { id: "solarized-light", name: "Solarized Light", swatches: ["#fdf6e3", "#eee8d5", "#657b83"] },
];

const THEME_IDS = THEMES.map((t) => t.id);

function getInitialTheme(): Theme {
  const stored = localStorage.getItem("mo-theme");
  if (THEME_IDS.includes(stored as Theme)) return stored as Theme;
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>(getInitialTheme);
  const [preview, setPreview] = useState<Theme | null>(null);
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", preview ?? theme);
  }, [preview, theme]);

  useEffect(() => {
    localStorage.setItem("mo-theme", theme);
  }, [theme]);

  useEffect(() => {
    if (!open) {
      setPreview(null);
      return;
    }
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open]);

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        className="flex items-center justify-center bg-transparent border border-gh-border rounded-md p-1.5 text-gh-header-text cursor-pointer transition-colors duration-150 hover:bg-gh-bg-hover"
        onClick={() => setOpen((v) => !v)}
        aria-label="Theme picker"
        title="Change theme"
      >
        <svg className="size-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.5}>
          <circle cx="12" cy="12" r="9" />
          <path d="M12 3a9 9 0 0 1 0 18V3z" fill="currentColor" stroke="none" />
        </svg>
      </button>

      <div
        className={`absolute right-0 top-full mt-1.5 w-44 bg-gh-bg-sidebar border border-gh-border rounded-lg shadow-xl z-10 py-1.5 transition-all duration-200 ease-in-out ${
          open ? "opacity-100 translate-y-0" : "opacity-0 -translate-y-2 pointer-events-none"
        }`}
        onMouseLeave={() => setPreview(null)}
      >
        <p className="px-3 pt-0.5 pb-1.5 text-[10px] font-semibold uppercase tracking-wider text-gh-text-secondary">
          Theme
        </p>
        {THEMES.map((t) => (
          <button
            key={t.id}
            className={`flex items-center gap-2.5 w-full px-3 py-1.5 border-none cursor-pointer text-left transition-colors duration-150 ${
              t.id === (preview ?? theme)
                ? "bg-gh-bg-active text-gh-text"
                : "bg-transparent text-gh-text-secondary hover:bg-gh-bg-hover hover:text-gh-text"
            }`}
            onMouseEnter={() => setPreview(t.id)}
            onClick={() => {
              setTheme(t.id);
              setPreview(null);
              setOpen(false);
            }}
          >
            <span className="text-xs font-medium flex-1">{t.name}</span>
            {t.id === theme && (
              <svg
                className="size-3.5 shrink-0"
                fill="none"
                stroke="currentColor"
                strokeWidth={2.5}
                viewBox="0 0 24 24"
              >
                <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
              </svg>
            )}
          </button>
        ))}
      </div>
    </div>
  );
}
