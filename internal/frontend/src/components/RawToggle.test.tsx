import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RawToggle } from "./RawToggle";

describe("RawToggle", () => {
  it("shows 'Show raw' title when isRaw is false", () => {
    render(<RawToggle isRaw={false} onToggle={() => {}} />);
    expect(screen.getByTitle("Show raw")).toBeInTheDocument();
  });

  it("shows 'Show rendered' title when isRaw is true", () => {
    render(<RawToggle isRaw={true} onToggle={() => {}} />);
    expect(screen.getByTitle("Show rendered")).toBeInTheDocument();
  });

  it("has aria-pressed false when not raw", () => {
    render(<RawToggle isRaw={false} onToggle={() => {}} />);
    const button = screen.getByRole("button", { name: "Raw view" });
    expect(button).toHaveAttribute("aria-pressed", "false");
  });

  it("has aria-pressed true when raw", () => {
    render(<RawToggle isRaw={true} onToggle={() => {}} />);
    const button = screen.getByRole("button", { name: "Raw view" });
    expect(button).toHaveAttribute("aria-pressed", "true");
  });

  it("calls onToggle when clicked", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    render(<RawToggle isRaw={false} onToggle={onToggle} />);

    await user.click(screen.getByRole("button"));
    expect(onToggle).toHaveBeenCalledOnce();
  });
});
