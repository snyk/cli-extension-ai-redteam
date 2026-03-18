import { describe, it, expect, vi, afterEach } from "vitest";
import { render, cleanup, screen, waitFor, fireEvent, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Form } from "antd";
import GoalStep from "./GoalStep";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const mockGoals = [
  { value: "Vulnerability Discovery", description: "Find security holes", display_order: 1, strategies: ["direct", "advanced"] },
  { value: "Compliance Testing", description: "Meet regulatory needs", display_order: 2, strategies: ["indirect"] },
  { value: "Basic Scan", description: "Simple scan", display_order: 3 },
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

/** Click the checkbox for a given goal by its value attribute. */
async function clickGoalCheckbox(goalValue: string) {
  const input = document.querySelector(`input[type="checkbox"][value="${goalValue}"]`) as HTMLElement;
  await userEvent.click(input);
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

  it("does not show strategy tags for unchecked goals", async () => {
    mockFetch();
    renderInForm();

    await waitFor(() => {
      expect(screen.getByText("Vulnerability Discovery")).toBeInTheDocument();
    });
    expect(screen.queryByText("direct")).not.toBeInTheDocument();
    expect(screen.queryByText("advanced")).not.toBeInTheDocument();
    expect(screen.queryByText("indirect")).not.toBeInTheDocument();
  });

  it("checking a goal pre-selects all its strategies", async () => {
    mockFetch();
    renderInForm();

    await waitFor(() => {
      expect(screen.getByText("Vulnerability Discovery")).toBeInTheDocument();
    });

    await clickGoalCheckbox("Vulnerability Discovery");

    await waitFor(() => {
      expect(screen.getByText("direct")).toBeInTheDocument();
    });
    expect(screen.getByText("advanced")).toBeInTheDocument();

    expect(screen.getByText("direct").closest(".ant-tag")).toHaveClass("ant-tag-checkable-checked");
    expect(screen.getByText("advanced").closest(".ant-tag")).toHaveClass("ant-tag-checkable-checked");
  });

  it("unchecking a goal removes its strategies", async () => {
    mockFetch();
    renderInForm();

    await waitFor(() => {
      expect(screen.getByText("Security")).toBeInTheDocument();
    });

    // Use profile click to reliably set initial checked state
    await act(async () => {
      fireEvent.click(screen.getByTestId("profile-card-security"));
    });
    await waitFor(() => {
      expect(screen.getByText("advanced")).toBeInTheDocument();
    });

    // Now uncheck the goal
    await clickGoalCheckbox("Vulnerability Discovery");
    await waitFor(() => {
      expect(screen.queryByText("direct")).not.toBeInTheDocument();
    });
    expect(screen.queryByText("advanced")).not.toBeInTheDocument();
  });

  it("profile click selects only profile strategies", async () => {
    mockFetch();
    renderInForm();

    await waitFor(() => {
      expect(screen.getByText("Fast")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("profile-card-security"));
    });

    await waitFor(() => {
      expect(screen.getByText("advanced")).toBeInTheDocument();
    });
    expect(screen.getByText("advanced").closest(".ant-tag")).toHaveClass("ant-tag-checkable-checked");
    expect(screen.getByText("direct").closest(".ant-tag")).not.toHaveClass("ant-tag-checkable-checked");
  });

  it("toggling a strategy tag clears profile selection", async () => {
    mockFetch();
    renderInForm();

    await waitFor(() => {
      expect(screen.getByText("Fast")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("profile-card-fast"));
    });

    await waitFor(() => {
      expect(screen.getByTestId("profile-card-fast")).toHaveClass("profile-card-selected");
    });

    await act(async () => {
      fireEvent.click(screen.getByText("direct"));
    });

    await waitFor(() => {
      expect(screen.getByTestId("profile-card-fast")).not.toHaveClass("profile-card-selected");
    });
  });

  it("goals without strategies render without tags", async () => {
    mockFetch();
    renderInForm();

    await waitFor(() => {
      expect(screen.getByText("Basic Scan")).toBeInTheDocument();
    });

    await clickGoalCheckbox("Basic Scan");

    expect(screen.getByText("Simple scan")).toBeInTheDocument();
    expect(document.querySelector(".ant-tag-checkable")).not.toBeInTheDocument();
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
