import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { WidthToggle } from "./WidthToggle";

describe("WidthToggle", () => {
  it("shows 'Wide view' title when narrow", () => {
    render(<WidthToggle isWide={false} onToggle={() => {}} />);
    expect(screen.getByTitle("Wide view")).toBeInTheDocument();
  });

  it("shows 'Narrow view' title when wide", () => {
    render(<WidthToggle isWide={true} onToggle={() => {}} />);
    expect(screen.getByTitle("Narrow view")).toBeInTheDocument();
  });

  it("has aria-pressed false when narrow", () => {
    render(<WidthToggle isWide={false} onToggle={() => {}} />);
    const button = screen.getByRole("button", { name: "Wide layout" });
    expect(button).toHaveAttribute("aria-pressed", "false");
  });

  it("has aria-pressed true when wide", () => {
    render(<WidthToggle isWide={true} onToggle={() => {}} />);
    const button = screen.getByRole("button", { name: "Wide layout" });
    expect(button).toHaveAttribute("aria-pressed", "true");
  });

  it("calls onToggle when clicked", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    render(<WidthToggle isWide={false} onToggle={onToggle} />);

    await user.click(screen.getByRole("button"));
    expect(onToggle).toHaveBeenCalledOnce();
  });
});
