import { describe, it, expect, afterEach } from "vitest";
import { render, cleanup, screen } from "@testing-library/react";
import { Form } from "antd";
import TargetDefinitionStep from "./TargetDefinitionStep";

afterEach(cleanup);

function renderInForm() {
  return render(
    <Form>
      <TargetDefinitionStep />
    </Form>,
  );
}

describe("TargetDefinitionStep", () => {
  it("renders target name input", () => {
    renderInForm();
    expect(screen.getByText("Target Name")).toBeInTheDocument();
  });

  it("renders target type select", () => {
    renderInForm();
    expect(screen.getByText("Target Type")).toBeInTheDocument();
  });
});
