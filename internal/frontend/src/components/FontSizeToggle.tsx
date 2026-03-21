import { useState, useEffect, useCallback } from "react";

const FONT_SIZE_KEY = "mo-font-size";
const MIN_SIZE = 12;
const MAX_SIZE = 24;
const DEFAULT_SIZE = 14;
const STEP = 2;

function getStoredFontSize(): number {
  try {
    const stored = localStorage.getItem(FONT_SIZE_KEY);
    if (stored) {
      const parsed = parseInt(stored, 10);
      if (!isNaN(parsed) && parsed >= MIN_SIZE && parsed <= MAX_SIZE) {
        return parsed;
      }
    }
  } catch {
    // localStorage may not be available
  }
  return DEFAULT_SIZE;
}

function storeFontSize(size: number): void {
  try {
    localStorage.setItem(FONT_SIZE_KEY, String(size));
  } catch {
    // localStorage may not be available
  }
}

interface FontSizeToggleProps {
  onSizeChange?: (size: number) => void;
}

export function FontSizeToggle({ onSizeChange }: FontSizeToggleProps) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [fontSize, setFontSize] = useState(getStoredFontSize);

  useEffect(() => {
    onSizeChange?.(fontSize);
  }, [fontSize, onSizeChange]);

  const handleDecrease = useCallback(() => {
    setFontSize((prev) => {
      const newSize = Math.max(MIN_SIZE, prev - STEP);
      storeFontSize(newSize);
      return newSize;
    });
  }, []);

  const handleIncrease = useCallback(() => {
    setFontSize((prev) => {
      const newSize = Math.min(MAX_SIZE, prev + STEP);
      storeFontSize(newSize);
      return newSize;
    });
  }, []);

  const handleAaClick = useCallback(() => {
    setIsExpanded((prev) => !prev);
  }, []);

  const buttonBaseClass =
    "flex items-center justify-center bg-transparent border border-gh-border rounded-md p-1.5 text-gh-text-secondary cursor-pointer transition-all duration-150 hover:bg-gh-bg-hover";

  return (
    <div className="flex flex-col gap-1">
      <div
        className={`overflow-hidden transition-all duration-200 ease-in-out ${
          isExpanded ? "max-h-10 opacity-100" : "max-h-0 opacity-0"
        }`}
      >
        <button
          type="button"
          className={`${buttonBaseClass} w-full`}
          onClick={handleDecrease}
          disabled={fontSize <= MIN_SIZE}
          aria-label="Decrease font size"
          title="Decrease font size"
        >
          <svg
            className="size-4"
            fill="none"
            stroke="currentColor"
            strokeWidth={2}
            viewBox="0 0 16 16"
          >
            <path d="M3 8h10" strokeLinecap="round" />
          </svg>
        </button>
      </div>

      <button
        type="button"
        className={buttonBaseClass}
        onClick={handleAaClick}
        aria-label={isExpanded ? "Close font size controls" : "Adjust font size"}
        title={isExpanded ? "Close font size controls" : "Adjust font size"}
      >
        <span className="text-sm font-semibold select-none">A<span className="text-[10px]">A</span></span>
      </button>

      <div
        className={`overflow-hidden transition-all duration-200 ease-in-out ${
          isExpanded ? "max-h-10 opacity-100" : "max-h-0 opacity-0"
        }`}
      >
        <button
          type="button"
          className={`${buttonBaseClass} w-full`}
          onClick={handleIncrease}
          disabled={fontSize >= MAX_SIZE}
          aria-label="Increase font size"
          title="Increase font size"
        >
          <svg
            className="size-4"
            fill="none"
            stroke="currentColor"
            strokeWidth={2}
            viewBox="0 0 16 16"
          >
            <path d="M3 8h10M8 3v10" strokeLinecap="round" />
          </svg>
        </button>
      </div>
    </div>
  );
}
