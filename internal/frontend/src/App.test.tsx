import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, waitFor } from "@testing-library/react";
import { App } from "./App";

const groups = [
  {
    name: "default",
    files: [
      { id: "aaa11111", name: "README.md", path: "/README.md", uploaded: false },
      { id: "bbb22222", name: "GUIDE.md", path: "/GUIDE.md", uploaded: false },
    ],
  },
];

function makeFetch(versionOverrides: object) {
  return vi.fn().mockImplementation((url: string) => {
    if (url === "/_/api/groups") {
      return Promise.resolve({ ok: true, json: () => Promise.resolve(groups) });
    }
    if (url === "/_/api/version") {
      return Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({ version: "0.0.0", revision: "test", ...versionOverrides }),
      });
    }
    if (url.includes("/_/api/files/") && url.includes("/content")) {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ content: "# Hello", baseDir: "/" }),
      });
    }
    return Promise.resolve({ ok: false });
  });
}

class MockEventSource {
  addEventListener = vi.fn();
  close = vi.fn();
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
}

beforeEach(() => {
  window.history.replaceState({}, "", "/");
  vi.stubGlobal("EventSource", MockEventSource);
  vi.spyOn(window.history, "replaceState");
  localStorage.clear();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("shareable URL sync", () => {
  it("reflects active file in ?file= param when shareable is true", async () => {
    vi.stubGlobal("fetch", makeFetch({ shareable: true }));

    render(<App />);

    await waitFor(() => {
      const calls = vi.mocked(window.history.replaceState).mock.calls;
      expect(calls.some(([, , url]) => String(url).includes("?file=aaa11111"))).toBe(true);
    });
  });

  it("does not add ?file= param when shareable is false", async () => {
    vi.stubGlobal("fetch", makeFetch({ shareable: false }));

    render(<App />);

    // Wait for version fetch to complete so we know shareable has been evaluated
    await waitFor(() => {
      expect(vi.mocked(fetch)).toHaveBeenCalledWith("/_/api/version");
    });

    const calls = vi.mocked(window.history.replaceState).mock.calls;
    expect(calls.every(([, , url]) => !String(url).includes("?file="))).toBe(true);
  });

  it("clears ?file= from URL after initial consume when shareable is false", async () => {
    window.history.pushState({}, "", "/?file=aaa11111");
    vi.stubGlobal("fetch", makeFetch({ shareable: false }));

    render(<App />);

    await waitFor(() => {
      const calls = vi.mocked(window.history.replaceState).mock.calls;
      expect(calls.some(([, , url]) => url === "/")).toBe(true);
    });
  });

  it("preserves ?file= in URL when shareable is true and file is restored from URL", async () => {
    window.history.pushState({}, "", "/?file=bbb22222");
    vi.stubGlobal("fetch", makeFetch({ shareable: true }));

    render(<App />);

    // bbb22222 is GUIDE.md — wait for it to become the active file
    await waitFor(() => {
      expect(document.title).toBe("GUIDE.md");
    });
    // aaa11111 (README.md) should never have been selected
    const calls = vi.mocked(window.history.replaceState).mock.calls;
    expect(calls.every(([, , url]) => !String(url).includes("?file=aaa11111"))).toBe(true);
  });
});
