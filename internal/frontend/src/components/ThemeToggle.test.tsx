import { describe, it, expect, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ThemeToggle } from "./ThemeToggle";

beforeEach(() => {
  localStorage.clear();
  document.documentElement.removeAttribute("data-theme");
});

describe("ThemeToggle", () => {
  it("renders toggle button", () => {
    render(<ThemeToggle />);
    expect(screen.getByTitle("Change theme")).toBeInTheDocument();
  });

  it("sets data-theme attribute on document from localStorage", () => {
    localStorage.setItem("mo-theme", "dark");
    render(<ThemeToggle />);
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
  });

  it("opens dropdown on click", async () => {
    const user = userEvent.setup();
    render(<ThemeToggle />);
    await user.click(screen.getByTitle("Change theme"));
    expect(screen.getByText("Light")).toBeInTheDocument();
    expect(screen.getByText("Dark")).toBeInTheDocument();
  });

  it("selects a theme from the dropdown", async () => {
    const user = userEvent.setup();
    localStorage.setItem("mo-theme", "light");
    render(<ThemeToggle />);

    await user.click(screen.getByTitle("Change theme"));
    await user.click(screen.getByText("Dark"));

    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    expect(localStorage.getItem("mo-theme")).toBe("dark");
  });

  it("closes dropdown after selecting a theme", async () => {
    const user = userEvent.setup();
    render(<ThemeToggle />);

    await user.click(screen.getByTitle("Change theme"));
    await user.click(screen.getByText("Dark"));

    // Dropdown should be hidden (pointer-events-none class)
    const dropdown = screen.getByText("Theme").closest("div");
    expect(dropdown?.className).toContain("pointer-events-none");
  });

  it("restores unknown stored theme to default", () => {
    localStorage.setItem("mo-theme", "invalid-theme");
    render(<ThemeToggle />);
    // Should fall back to system preference or light; data-theme should be set to a valid value
    const attr = document.documentElement.getAttribute("data-theme");
    expect(["light", "dark"]).toContain(attr);
  });
});
