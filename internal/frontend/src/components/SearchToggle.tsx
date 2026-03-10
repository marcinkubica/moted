interface SearchToggleProps {
  isOpen: boolean;
  onToggle: () => void;
}

export function SearchToggle({ isOpen, onToggle }: SearchToggleProps) {
  return (
    <button
      type="button"
      className={`flex items-center justify-center bg-transparent border border-gh-border rounded-md p-1.5 cursor-pointer transition-colors duration-150 hover:bg-gh-bg-hover ${
        isOpen ? "text-gh-header-text bg-gh-bg-hover" : "text-gh-header-text"
      }`}
      onClick={onToggle}
      aria-label="Search"
      aria-expanded={isOpen}
      title={isOpen ? "Close search" : "Search files"}
    >
      <svg
        className="size-5"
        fill="none"
        stroke="currentColor"
        strokeWidth={1.5}
        viewBox="0 0 24 24"
      >
        <circle cx="11" cy="11" r="7" />
        <line x1="16.5" y1="16.5" x2="21" y2="21" strokeLinecap="round" />
      </svg>
    </button>
  );
}
