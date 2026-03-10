interface WidthToggleProps {
  isWide: boolean;
  onToggle: () => void;
}

// Inward arrows →← (compress)
const COMPRESS_PATH = "M2 12h8m0 0l-4-5M10 12l-4 5M22 12h-8m0 0l4-5M14 12l4 5";
// Outward arrows ←→ (expand)
const EXPAND_PATH = "M10 12H2m0 0l4-5M2 12l4 5M14 12h8m0 0l-4-5M22 12l-4 5";

export function WidthToggle({ isWide, onToggle }: WidthToggleProps) {
  return (
    <button
      type="button"
      className="flex items-center justify-center bg-transparent border border-gh-border rounded-md p-1.5 text-gh-header-text cursor-pointer transition-colors duration-150 hover:bg-gh-bg-hover"
      onClick={onToggle}
      aria-label="Wide layout"
      aria-pressed={isWide}
      title={isWide ? "Narrow view" : "Wide view"}
    >
      <svg
        className="size-5"
        fill="none"
        stroke="currentColor"
        strokeWidth={1.5}
        viewBox="0 0 24 24"
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          d={isWide ? COMPRESS_PATH : EXPAND_PATH}
        />
      </svg>
    </button>
  );
}
