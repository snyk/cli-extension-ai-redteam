import { describe, it, expect, vi, afterEach } from "vitest";
import { render, cleanup, fireEvent, within } from "@testing-library/react";
import Sidebar, { steps } from "./Sidebar";

afterEach(cleanup);

describe("Sidebar", () => {
  const defaultProps = {
    activeStep: "target-type",
    onStepClick: vi.fn(),
    configPath: null as string | null,
  };

  it("renders all 6 step labels", () => {
    const { container } = render(<Sidebar {...defaultProps} />);
    for (const step of steps) {
      expect(within(container).getByText(step.label)).toBeInTheDocument();
    }
  });

  it("applies active class to the current step", () => {
    const { container } = render(<Sidebar {...defaultProps} activeStep="goal" />);
    const buttons = container.querySelectorAll("button");
    const goalButton = Array.from(buttons).find((b) => b.textContent?.includes("Goals"));
    expect(goalButton?.className).toMatch(/active/);
  });

  it("does not apply active class to non-active steps", () => {
    const { container } = render(<Sidebar {...defaultProps} activeStep="goal" />);
    const buttons = container.querySelectorAll("button");
    const targetButton = Array.from(buttons).find((b) => b.textContent?.includes("Target Type"));
    expect(targetButton?.className).not.toMatch(/active/);
  });

  it("calls onStepClick with correct key on click", () => {
    const onStepClick = vi.fn();
    const { container } = render(<Sidebar {...defaultProps} onStepClick={onStepClick} />);
    fireEvent.click(within(container).getByText("Strategies"));
    expect(onStepClick).toHaveBeenCalledWith("strategies");
  });

  it('shows "New Configuration" when configPath is null', () => {
    const { container } = render(<Sidebar {...defaultProps} configPath={null} />);
    expect(within(container).getByText("New Configuration")).toBeInTheDocument();
  });

  it('shows "Configuring <path>" when configPath is provided', () => {
    const { container } = render(<Sidebar {...defaultProps} configPath="redteam.yaml" />);
    expect(within(container).getByText("Configuring redteam.yaml")).toBeInTheDocument();
  });
});
