import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import YamlHighlight from "./YamlHighlight";

describe("YamlHighlight", () => {
  it("renders without crashing", () => {
    render(<YamlHighlight content="key: value" />);
    const pre = screen.getByText((_content, element) => element?.tagName === "PRE");
    expect(pre).toBeInTheDocument();
    expect(pre).toHaveClass("yaml-highlight");
  });

  it("applies correct CSS classes to keys and string values", () => {
    const { container } = render(<YamlHighlight content="name: hello" />);
    const keySpan = container.querySelector(".yaml-key");
    expect(keySpan).toHaveTextContent("name");
    const colonSpan = container.querySelector(".yaml-colon");
    expect(colonSpan).toHaveTextContent(":");
  });

  it("applies yaml-string class to quoted values", () => {
    const { container } = render(<YamlHighlight content="name: 'quoted'" />);
    const stringSpan = container.querySelector(".yaml-string");
    expect(stringSpan).toHaveTextContent("'quoted'");
  });

  it("applies yaml-bool class to boolean values", () => {
    const { container } = render(<YamlHighlight content="enabled: true" />);
    const boolSpan = container.querySelector(".yaml-bool");
    expect(boolSpan).toHaveTextContent("true");
  });

  it("applies yaml-number class to numeric values", () => {
    const { container } = render(<YamlHighlight content="count: 42" />);
    const numSpan = container.querySelector(".yaml-number");
    expect(numSpan).toHaveTextContent("42");
  });

  it("escapes HTML entities in content", () => {
    const { container } = render(<YamlHighlight content="tag: <script>&alert" />);
    const pre = container.querySelector("pre");
    expect(pre?.innerHTML).not.toContain("<script>");
    expect(pre?.innerHTML).toContain("&lt;script&gt;");
    expect(pre?.innerHTML).toContain("&amp;alert");
  });

  it("highlights comment lines", () => {
    const { container } = render(<YamlHighlight content="# this is a comment" />);
    const commentSpan = container.querySelector(".yaml-comment");
    expect(commentSpan).toHaveTextContent("# this is a comment");
  });

  it("highlights list items with dash", () => {
    const { container } = render(<YamlHighlight content="  - item_one" />);
    const dashSpan = container.querySelector(".yaml-dash");
    expect(dashSpan).toBeInTheDocument();
    expect(dashSpan?.textContent).toBe("- ");
  });
});
