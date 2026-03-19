import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { render, cleanup, screen, fireEvent, waitFor } from "@testing-library/react";
import SetupPage from "./SetupPage";

// GoalStep fetches on mount — stub fetch globally
beforeEach(() => {
  vi.spyOn(globalThis, "fetch").mockImplementation((input) => {
    const url = typeof input === "string" ? input : (input as Request).url;

    if (url === "/api/goals") {
      return Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve([
            { value: "system_prompt_extraction", description: "Extract system prompt", display_order: 1 },
          ]),
      } as Response);
    }

    if (url === "/api/strategies") {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve([]),
      } as Response);
    }

    if (url === "/api/profiles") {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve([]),
      } as Response);
    }

    if (url === "/api/config" && (!input || typeof input === "string" || (input as Request).method !== "POST")) {
      return Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            config_path: null,
            config: null,
          }),
      } as Response);
    }

    if (url === "/api/config") {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ yaml: "target:\n  name: test\n" }),
      } as Response);
    }

    return Promise.reject(new Error(`Unmocked fetch: ${url}`));
  });

  // Mock Form.useWatch for TestConnection
  const { Form } = require("antd");
  vi.spyOn(Form, "useWatch").mockImplementation(() => undefined);
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const defaultProps = {
  activeStep: "target-type",
  onStepChange: vi.fn(),
  onConfigPathLoaded: vi.fn(),
};

describe("SetupPage", () => {
  it("renders step title for target-type", async () => {
    render(<SetupPage {...defaultProps} />);
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Target Type" })).toBeInTheDocument();
    });
  });

  it("renders Next button on non-review steps", () => {
    render(<SetupPage {...defaultProps} />);
    expect(screen.getByRole("button", { name: /next/i })).toBeInTheDocument();
  });

  it("does not render Back button on first step", () => {
    render(<SetupPage {...defaultProps} />);
    expect(screen.queryByRole("button", { name: /^back$/i })).not.toBeInTheDocument();
  });

  it("renders Back button on non-first step", () => {
    render(<SetupPage {...defaultProps} activeStep="target-config" />);
    expect(screen.getByRole("button", { name: /^back$/i })).toBeInTheDocument();
  });

  it("calls onStepChange when Back is clicked", () => {
    const onStepChange = vi.fn();
    render(<SetupPage {...defaultProps} activeStep="target-config" onStepChange={onStepChange} />);
    fireEvent.click(screen.getByRole("button", { name: /^back$/i }));
    expect(onStepChange).toHaveBeenCalledWith("target-type");
  });

  it("renders Download Configuration button on review step", () => {
    render(<SetupPage {...defaultProps} activeStep="review" />);
    expect(screen.getByRole("button", { name: /download configuration/i })).toBeInTheDocument();
  });

  it("renders Review Configuration text on goal step (last before review)", () => {
    render(<SetupPage {...defaultProps} activeStep="goal" />);
    expect(screen.getByRole("button", { name: /review configuration/i })).toBeInTheDocument();
  });

  it("fetches /api/config on mount and calls onConfigPathLoaded", async () => {
    const onConfigPathLoaded = vi.fn();
    render(<SetupPage {...defaultProps} onConfigPathLoaded={onConfigPathLoaded} />);
    await waitFor(() => {
      expect(onConfigPathLoaded).toHaveBeenCalledWith(null);
    });
  });

  it("loads existing config from /api/config on mount", async () => {
    vi.mocked(globalThis.fetch).mockImplementation((input) => {
      const url = typeof input === "string" ? input : (input as Request).url;

      if (url === "/api/config") {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({
              config_path: "existing.yaml",
              config: {
                target: {
                  name: "Loaded API",
                  type: "http",
                  settings: { url: "https://loaded.com" },
                },
                goals: ["system_prompt_extraction"],
              },
            }),
        } as Response);
      }

      if (url === "/api/goals" || url === "/api/profiles") {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve([]),
        } as Response);
      }

      return Promise.reject(new Error(`Unmocked fetch: ${url}`));
    });

    const onConfigPathLoaded = vi.fn();
    render(<SetupPage {...defaultProps} onConfigPathLoaded={onConfigPathLoaded} />);

    await waitFor(() => {
      expect(onConfigPathLoaded).toHaveBeenCalledWith("existing.yaml");
    });
  });

  it("shows download modal when Download Configuration is clicked", async () => {
    render(<SetupPage {...defaultProps} activeStep="review" />);
    fireEvent.click(screen.getByRole("button", { name: /download configuration/i }));
    await waitFor(() => {
      expect(screen.getByText("Save as")).toBeInTheDocument();
    });
  });

  it("shows validation error when clicking Review Configuration without goals", async () => {
    render(<SetupPage {...defaultProps} activeStep="goal" />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /review configuration/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /review configuration/i }));

    await waitFor(() => {
      expect(screen.getByText(/select at least one goal/i)).toBeInTheDocument();
    });
  });

  it("shows validation error when clicking Next without target name", async () => {
    render(<SetupPage {...defaultProps} activeStep="target-type" />);

    fireEvent.click(screen.getByRole("button", { name: /next/i }));

    await waitFor(() => {
      expect(screen.getByText(/select target name/i)).toBeInTheDocument();
    });
  });

  it("validation error does not show on a different step", async () => {
    const { rerender } = render(<SetupPage {...defaultProps} activeStep="goal" />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /review configuration/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /review configuration/i }));

    await waitFor(() => {
      expect(screen.getByText(/select at least one goal/i)).toBeInTheDocument();
    });

    rerender(<SetupPage {...defaultProps} activeStep="target-type" />);

    expect(screen.queryByText(/select at least one goal/i)).not.toBeInTheDocument();
  });
});
