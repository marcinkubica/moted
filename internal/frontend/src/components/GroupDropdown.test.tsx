import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GroupDropdown } from "./GroupDropdown";
import type { Group } from "../hooks/useApi";

const defaultGroup: Group = {
  name: "default",
  files: [{ id: "aaa11111", name: "a.md", path: "/a.md" }],
};

const docsGroup: Group = {
  name: "docs",
  files: [{ id: "bbb22222", name: "b.md", path: "/b.md" }],
};

const designGroup: Group = {
  name: "design",
  files: [{ id: "ccc33333", name: "c.md", path: "/c.md" }],
};

describe("GroupDropdown", () => {
  it("renders nothing for a single default group", () => {
    const { container } = render(
      <GroupDropdown groups={[defaultGroup]} activeGroup="default" onGroupChange={() => {}} />,
    );
    expect(container.innerHTML).toBe("");
  });

  it("renders group name without dropdown for a single non-default group", () => {
    render(<GroupDropdown groups={[docsGroup]} activeGroup="docs" onGroupChange={() => {}} />);
    expect(screen.getByText("docs")).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("renders dropdown button for multiple groups", () => {
    render(
      <GroupDropdown
        groups={[defaultGroup, docsGroup]}
        activeGroup="default"
        onGroupChange={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: "Select group" })).toBeInTheDocument();
  });

  it("shows group name in button for non-default active group", () => {
    render(
      <GroupDropdown
        groups={[defaultGroup, docsGroup]}
        activeGroup="docs"
        onGroupChange={() => {}}
      />,
    );
    expect(screen.getByText("docs", { selector: "span.font-bold" })).toBeInTheDocument();
  });

  it("opens dropdown on click and shows all groups", async () => {
    const user = userEvent.setup();
    render(
      <GroupDropdown
        groups={[defaultGroup, docsGroup, designGroup]}
        activeGroup="default"
        onGroupChange={() => {}}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Select group" }));
    expect(screen.getByText("(default)")).toBeInTheDocument();
    expect(screen.getByText("docs")).toBeInTheDocument();
    expect(screen.getByText("design")).toBeInTheDocument();
  });

  it("calls onGroupChange when a group is selected", async () => {
    const user = userEvent.setup();
    const onGroupChange = vi.fn();
    render(
      <GroupDropdown
        groups={[defaultGroup, docsGroup]}
        activeGroup="default"
        onGroupChange={onGroupChange}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Select group" }));
    await user.click(screen.getByText("docs"));
    expect(onGroupChange).toHaveBeenCalledWith("docs");
  });

  it("closes dropdown after selecting a group", async () => {
    const user = userEvent.setup();
    render(
      <GroupDropdown
        groups={[defaultGroup, docsGroup]}
        activeGroup="default"
        onGroupChange={() => {}}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Select group" }));
    expect(screen.getByText("docs")).toBeInTheDocument();

    await user.click(screen.getByText("docs"));
    // After selection, dropdown should be hidden (opacity-0), not removed from DOM
    const dropdown = screen.getByText("(default)").closest("div[class*='absolute']");
    expect(dropdown).toHaveClass("opacity-0");
  });
});
