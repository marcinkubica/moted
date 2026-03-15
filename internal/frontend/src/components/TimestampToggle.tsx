interface TimestampToggleProps {
  isActive: boolean;
  onToggle: () => void;
}

export function TimestampToggle({ isActive, onToggle }: TimestampToggleProps) {
  return (
    <button
      type="button"
      className={`flex items-center justify-center bg-transparent border border-gh-border rounded-md p-1.5 cursor-pointer transition-colors duration-150 hover:bg-gh-bg-hover ${
        isActive ? "text-gh-header-text bg-gh-bg-hover" : "text-gh-header-text"
      }`}
      onClick={onToggle}
      aria-label="Toggle timestamps"
      aria-pressed={isActive}
      title={isActive ? "Hide file timestamps" : "Show file timestamps"}
    >
      <svg
        className="size-5"
        fill="none"
        stroke="currentColor"
        strokeWidth={1.5}
        viewBox="0 0 24 24"
      >
        <circle cx="12" cy="12" r="9" />
        <polyline points="12,7 12,12 15.5,14" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    </button>
  );
}
