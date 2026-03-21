export type SortMode = "manual" | "alpha-asc" | "alpha-desc" | "time-asc" | "time-desc";

const MODES: { key: SortMode; label: string }[] = [
  { key: "manual", label: "Manual order" },
  { key: "alpha-asc", label: "Name A→Z" },
  { key: "alpha-desc", label: "Name Z→A" },
  { key: "time-asc", label: "Oldest first" },
  { key: "time-desc", label: "Newest first" },
];

const STORAGE_KEY = "mo-sort-mode";

function loadSortMode(): SortMode {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    if (v && MODES.some((m) => m.key === v)) return v as SortMode;
  } catch {
    /* ignore */
  }
  return "manual";
}

export function getInitialSortMode(): SortMode {
  return loadSortMode();
}

interface SortToggleProps {
  mode: SortMode;
  onToggle: () => void;
}

export function SortToggle({ mode, onToggle }: SortToggleProps) {
  const label = MODES.find((m) => m.key === mode)?.label ?? "Sort";

  return (
    <div className="relative">
      <button
        type="button"
        className="flex items-center justify-center bg-transparent border border-gh-border rounded-md p-1.5 cursor-pointer text-gh-header-text transition-colors duration-150 hover:bg-gh-bg-hover"
        onClick={onToggle}
        title={label}
        aria-label={label}
      >
        {mode === "manual" ? (
          <svg className="size-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" d="M3 7.5 7.5 3m0 0L12 7.5M7.5 3v13.5m13.5 0L16.5 21m0 0L12 16.5m4.5 4.5V7.5" />
          </svg>
        ) : mode === "alpha-asc" ? (
          <svg className="size-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" d="M3 4.5h14.25M3 9h9.75M3 13.5h5.25m8.25 3V6.75m0 0 3 3m-3-3-3 3" />
          </svg>
        ) : mode === "alpha-desc" ? (
          <svg className="size-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" d="M3 4.5h14.25M3 9h9.75M3 13.5h9.75m4.5-7.5v13.5m0 0-3-3m3 3 3-3" />
          </svg>
        ) : mode === "time-asc" ? (
          <svg className="size-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
            <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 13.5 19.5 7.5m0 0 2 2m-2-2-2 2" opacity="0.5" />
          </svg>
        ) : (
          <svg className="size-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
            <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 7.5 19.5 13.5m0 0 2-2m-2 2-2-2" opacity="0.5" />
          </svg>
        )}
      </button>
    </div>
  );
}

export function cycleSortMode(current: SortMode): SortMode {
  const keys = MODES.map((m) => m.key);
  const idx = keys.indexOf(current);
  return keys[(idx + 1) % keys.length];
}

export function saveSortMode(mode: SortMode) {
  try {
    localStorage.setItem(STORAGE_KEY, mode);
  } catch {
    /* ignore */
  }
}
