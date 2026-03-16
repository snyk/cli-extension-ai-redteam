import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { render, cleanup, screen, waitFor } from "@testing-library/react";
import { Form } from "antd";
import GoalStep from "./GoalStep";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const mockGoals = [
  { value: "Vulnerability Discovery", description: "Find security holes", display_order: 1 },
  { value: "Compliance Testing", description: "Meet regulatory needs", display_order: 2 },
];

function renderInForm() {
  return render(
    <Form>
      <GoalStep />
    </Form>,
  );
}

describe("GoalStep", () => {
  it("shows spinner while loading", () => {
    vi.spyOn(globalThis, "fetch").mockReturnValue(new Promise(() => {}));
    renderInForm();
    expect(document.querySelector(".ant-spin")).toBeInTheDocument();
  });

  it("renders goal checkboxes after successful fetch", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockGoals),
    } as Response);

    renderInForm();

    await waitFor(() => {
      expect(screen.getByText("Vulnerability Discovery")).toBeInTheDocument();
    });
    expect(screen.getByText("Compliance Testing")).toBeInTheDocument();
    expect(screen.getByText("Find security holes")).toBeInTheDocument();
    expect(screen.getByText("Meet regulatory needs")).toBeInTheDocument();
  });

  it("shows error alert when fetch rejects", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("Network error"));

    renderInForm();

    await waitFor(() => {
      expect(screen.getByText("Failed to load goals")).toBeInTheDocument();
    });
    expect(screen.getByText("Network error")).toBeInTheDocument();
  });

  it("shows error alert on non-ok HTTP response", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: false,
      status: 500,
    } as Response);

    renderInForm();

    await waitFor(() => {
      expect(screen.getByText("Failed to load goals")).toBeInTheDocument();
    });
    expect(screen.getByText("HTTP 500")).toBeInTheDocument();
  });
});
