import { describe, it, expect, afterEach } from "vitest";
import { render, cleanup, screen } from "@testing-library/react";
import { Form } from "antd";
import AppContextStep from "./AppContextStep";

afterEach(cleanup);

function renderInForm() {
  return render(
    <Form>
      <AppContextStep />
    </Form>,
  );
}

describe("AppContextStep", () => {
  it("renders purpose textarea", () => {
    renderInForm();
    expect(screen.getByText("Purpose")).toBeInTheDocument();
  });

  it("renders ground truth section heading", () => {
    renderInForm();
    expect(screen.getByText("Ground Truth")).toBeInTheDocument();
  });

  it("renders ground truth info alert", () => {
    renderInForm();
    expect(
      screen.getByText(/must exactly match what the target is configured with/),
    ).toBeInTheDocument();
  });

  it("renders system prompt field", () => {
    renderInForm();
    expect(screen.getByText("System Prompt")).toBeInTheDocument();
  });

  it("renders tools field", () => {
    renderInForm();
    expect(screen.getByText("Tools")).toBeInTheDocument();
  });
});
