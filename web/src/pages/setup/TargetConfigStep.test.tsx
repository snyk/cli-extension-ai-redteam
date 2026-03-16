import { describe, it, expect, vi, afterEach } from "vitest";
import { render, cleanup, screen } from "@testing-library/react";
import { Form } from "antd";
import TargetConfigStep from "./TargetConfigStep";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

// Mock Form.useWatch so TestConnection's canTest logic doesn't break in happy-dom
vi.spyOn(Form, "useWatch").mockImplementation(() => undefined);

function renderInForm() {
  return render(
    <Form>
      <TargetConfigStep />
    </Form>,
  );
}

describe("TargetConfigStep", () => {
  it("renders target URL field", () => {
    renderInForm();
    expect(screen.getByText("Target URL")).toBeInTheDocument();
  });

  it("renders request body template field", () => {
    renderInForm();
    expect(screen.getByText("Request Body Template")).toBeInTheDocument();
  });

  it("renders response selector field", () => {
    renderInForm();
    expect(screen.getByText("Response Selector")).toBeInTheDocument();
  });

  it("renders headers section", () => {
    renderInForm();
    expect(screen.getByText("Headers")).toBeInTheDocument();
  });

  it("renders test connection button", () => {
    renderInForm();
    expect(screen.getByRole("button", { name: /test connection/i })).toBeInTheDocument();
  });

  it("renders add header button", () => {
    renderInForm();
    expect(screen.getByRole("button", { name: /add header/i })).toBeInTheDocument();
  });
});
