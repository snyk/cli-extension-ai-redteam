import { describe, it, expect, afterEach } from "vitest";
import { render, cleanup, screen, fireEvent } from "@testing-library/react";
import { Form } from "antd";
import HeadersEditor from "./HeadersEditor";

afterEach(cleanup);

function renderInForm() {
  return render(
    <Form>
      <HeadersEditor />
    </Form>,
  );
}

describe("HeadersEditor", () => {
  it("renders add header button", () => {
    renderInForm();
    expect(screen.getByRole("button", { name: /add header/i })).toBeInTheDocument();
  });

  it("starts with no header rows", () => {
    renderInForm();
    expect(screen.queryByPlaceholderText(/header name/i)).not.toBeInTheDocument();
  });

  it("adds a header row when clicking add", () => {
    renderInForm();
    fireEvent.click(screen.getByRole("button", { name: /add header/i }));
    expect(screen.getByPlaceholderText(/Header name/)).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/Header value/)).toBeInTheDocument();
  });

  it("adds multiple header rows", () => {
    renderInForm();
    const addBtn = screen.getByRole("button", { name: /add header/i });
    fireEvent.click(addBtn);
    fireEvent.click(addBtn);
    expect(screen.getAllByPlaceholderText(/Header name/)).toHaveLength(2);
  });

  it("removes a header row when clicking delete", () => {
    renderInForm();
    fireEvent.click(screen.getByRole("button", { name: /add header/i }));
    expect(screen.getByPlaceholderText(/Header name/)).toBeInTheDocument();

    // The delete button has a DeleteOutlined icon
    const deleteBtn = screen.getByRole("button", { name: /delete/i });
    fireEvent.click(deleteBtn);
    expect(screen.queryByPlaceholderText(/Header name/)).not.toBeInTheDocument();
  });
});
