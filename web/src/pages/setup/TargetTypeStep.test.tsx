import { describe, it, expect, afterEach } from "vitest";
import { render, cleanup, screen } from "@testing-library/react";
import { Form } from "antd";
import TargetTypeStep from "./TargetTypeStep";

afterEach(cleanup);

function renderInForm() {
  return render(
    <Form>
      <TargetTypeStep />
    </Form>,
  );
}

describe("TargetTypeStep", () => {
  it("renders target name input", () => {
    renderInForm();
    expect(screen.getByText("Target Name")).toBeInTheDocument();
  });

  it("renders target type select", () => {
    renderInForm();
    expect(screen.getByText("Target Type")).toBeInTheDocument();
  });
});
