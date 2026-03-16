import { describe, it, expect, vi, afterEach } from "vitest";
import { render, cleanup, screen, waitFor } from "@testing-library/react";
import { Form } from "antd";
import StrategiesStep from "./StrategiesStep";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const mockStrategies = [
  { value: "OWASP Top 10", description: "Common web vulnerabilities", display_order: 1 },
  { value: "Custom Payloads", description: "User-defined attack patterns", display_order: 2 },
];

function renderInForm() {
  return render(
    <Form>
      <StrategiesStep />
    </Form>,
  );
}

describe("StrategiesStep", () => {
  it("shows spinner while loading", () => {
    vi.spyOn(globalThis, "fetch").mockReturnValue(new Promise(() => {}));
    renderInForm();
    expect(document.querySelector(".ant-spin")).toBeInTheDocument();
  });

  it("renders strategy checkboxes after successful fetch", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockStrategies),
    } as Response);

    renderInForm();

    await waitFor(() => {
      expect(screen.getByText("OWASP Top 10")).toBeInTheDocument();
    });
    expect(screen.getByText("Custom Payloads")).toBeInTheDocument();
    expect(screen.getByText("Common web vulnerabilities")).toBeInTheDocument();
    expect(screen.getByText("User-defined attack patterns")).toBeInTheDocument();
  });

  it("shows error alert when fetch rejects", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("Network error"));

    renderInForm();

    await waitFor(() => {
      expect(screen.getByText("Failed to load strategies")).toBeInTheDocument();
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
      expect(screen.getByText("Failed to load strategies")).toBeInTheDocument();
    });
    expect(screen.getByText("HTTP 500")).toBeInTheDocument();
  });
});
