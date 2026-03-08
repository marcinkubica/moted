import { useCallback, useEffect, useRef, useState } from "react";
import { addFile, uploadFile } from "./useApi";

function extractFilePaths(dataTransfer: DataTransfer): string[] {
  // Chrome/Edge: text/uri-list
  const uriList = dataTransfer.getData("text/uri-list");
  if (uriList) {
    return uriList
      .split(/\r?\n/)
      .filter((line) => line.startsWith("file://"))
      .map((uri) => decodeURIComponent(new URL(uri).pathname));
  }

  // Firefox: text/x-moz-url (tab-separated URL\nTitle pairs)
  const mozUrl = dataTransfer.getData("text/x-moz-url");
  if (mozUrl) {
    return mozUrl
      .split(/\r?\n/)
      .filter((line) => line.startsWith("file://"))
      .map((uri) => decodeURIComponent(new URL(uri).pathname));
  }

  return [];
}

function isMarkdown(name: string): boolean {
  const lower = name.toLowerCase();
  return lower.endsWith(".md") || lower.endsWith(".markdown");
}

export function useFileDrop(activeGroup: string): { isDragging: boolean } {
  const [isDragging, setIsDragging] = useState(false);
  const dragCounter = useRef(0);

  const handleDragEnter = useCallback((e: DragEvent) => {
    e.preventDefault();
    dragCounter.current++;
    if (dragCounter.current === 1) {
      setIsDragging(true);
    }
  }, []);

  const handleDragOver = useCallback((e: DragEvent) => {
    e.preventDefault();
  }, []);

  const handleDragLeave = useCallback((e: DragEvent) => {
    e.preventDefault();
    dragCounter.current--;
    if (dragCounter.current === 0) {
      setIsDragging(false);
    }
  }, []);

  const handleDrop = useCallback(
    async (e: DragEvent) => {
      e.preventDefault();
      dragCounter.current = 0;
      setIsDragging(false);

      if (!e.dataTransfer) return;

      // Pattern 1: file:// URI available (Firefox)
      const paths = extractFilePaths(e.dataTransfer);
      if (paths.length > 0) {
        const mdPaths = paths.filter(isMarkdown);
        for (const p of mdPaths) {
          addFile(p, activeGroup);
        }
        return;
      }

      // Pattern 2: File objects only (Chrome/Edge) - upload content
      const files = e.dataTransfer.files;
      for (let i = 0; i < files.length; i++) {
        const file = files[i];
        if (isMarkdown(file.name)) {
          const content = await file.text();
          uploadFile(file.name, content, activeGroup);
        }
      }
    },
    [activeGroup],
  );

  useEffect(() => {
    document.addEventListener("dragenter", handleDragEnter);
    document.addEventListener("dragover", handleDragOver);
    document.addEventListener("dragleave", handleDragLeave);
    document.addEventListener("drop", handleDrop);
    return () => {
      document.removeEventListener("dragenter", handleDragEnter);
      document.removeEventListener("dragover", handleDragOver);
      document.removeEventListener("dragleave", handleDragLeave);
      document.removeEventListener("drop", handleDrop);
    };
  }, [handleDragEnter, handleDragOver, handleDragLeave, handleDrop]);

  return { isDragging };
}
