import { describe, it, expect, vi, afterEach } from "vitest";
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

const mockProfiles = [
  {
    id: "fast",
    name: "Fast",
    description: "Quick baseline check",
    entries: [
      { goal: "Vulnerability Discovery", strategy: "direct" },
      { goal: "Compliance Testing", strategy: "indirect" },
    ],
  },
  {
    id: "security",
    name: "Security",
    description: "Deep security scan",
    entries: [{ goal: "Vulnerability Discovery", strategy: "advanced" }],
  },
];

function mockFetch(goals: any[] = mockGoals, profiles: any[] = mockProfiles) {
  vi.spyOn(globalThis, "fetch").mockImplementation((input) => {
    const url = typeof input === "string" ? input : (input as Request).url;
    if (url === "/api/goals") {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(goals),
      } as Response);
    }
    if (url === "/api/profiles") {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(profiles),
      } as Response);
    }
    return Promise.reject(new Error(`Unmocked fetch: ${url}`));
  });
}

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
    mockFetch();
    renderInForm();

    await waitFor(() => {
      expect(screen.getByText("Vulnerability Discovery")).toBeInTheDocument();
    });
    expect(screen.getByText("Compliance Testing")).toBeInTheDocument();
    expect(screen.getByText("Find security holes")).toBeInTheDocument();
    expect(screen.getByText("Meet regulatory needs")).toBeInTheDocument();
  });

  it("renders profile cards", async () => {
    mockFetch();
    renderInForm();

    await waitFor(() => {
      expect(screen.getByText("Fast")).toBeInTheDocument();
    });
    expect(screen.getByText("Quick baseline check")).toBeInTheDocument();
    expect(screen.getByText("Security")).toBeInTheDocument();
    expect(screen.getByText("Deep security scan")).toBeInTheDocument();
  });

  it("does not render profile cards when no profiles", async () => {
    mockFetch(mockGoals, []);
    renderInForm();

    await waitFor(() => {
      expect(screen.getByText("Vulnerability Discovery")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("profile-card-fast")).not.toBeInTheDocument();
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
