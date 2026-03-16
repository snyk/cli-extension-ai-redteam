import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { render, cleanup, screen, fireEvent, waitFor } from "@testing-library/react";
import { Form } from "antd";
import TestConnection, { isValidHttpUrl } from "./TestConnection";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

// Form.useWatch doesn't trigger re-renders in happy-dom, so we mock it
// to control the disabled state of the button directly.
let useWatchReturn: Record<string, unknown> = {};

beforeEach(() => {
  useWatchReturn = {};
  vi.spyOn(Form, "useWatch").mockImplementation((namePath) => {
    const key = Array.isArray(namePath) ? namePath.join(".") : String(namePath);
    return useWatchReturn[key];
  });
});

const defaultSettings = {
  url: "https://example.com",
  request_body_template: '{"prompt":"{{prompt}}"}',
  headers: [],
};

function renderComponent(overrides: Partial<typeof defaultSettings> = {}) {
  const settings = { ...defaultSettings, ...overrides };

  // Feed the mocked useWatch
  useWatchReturn["target.settings.url"] = settings.url;
  useWatchReturn["target.settings.request_body_template"] = settings.request_body_template;

  return render(
    <Form
      initialValues={{ target: { settings } }}
    >
      <TestConnection />
    </Form>,
  );
}

describe("isValidHttpUrl", () => {
  it("accepts http url", () => expect(isValidHttpUrl("http://localhost:8080")).toBe(true));
  it("accepts https url", () => expect(isValidHttpUrl("https://api.example.com/v1")).toBe(true));
  it("rejects empty string", () => expect(isValidHttpUrl("")).toBe(false));
  it("rejects undefined", () => expect(isValidHttpUrl(undefined)).toBe(false));
  it("rejects non-http protocol", () => expect(isValidHttpUrl("ftp://files.example.com")).toBe(false));
  it("rejects plain text", () => expect(isValidHttpUrl("not-a-url")).toBe(false));
});

describe("TestConnection", () => {
  it("renders enabled button when url and body are valid", () => {
    renderComponent();
    expect(screen.getByRole("button", { name: /test connection/i })).not.toBeDisabled();
  });

  it("renders disabled button when url is empty", () => {
    renderComponent({ url: "" });
    expect(screen.getByRole("button", { name: /test connection/i })).toBeDisabled();
  });

  it("renders disabled button when body is empty", () => {
    renderComponent({ request_body_template: "" });
    expect(screen.getByRole("button", { name: /test connection/i })).toBeDisabled();
  });

  it("shows success alert after successful ping", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          success: true,
          response: "Hello world",
          suggestion: "Target is reachable.",
        }),
    } as Response);

    renderComponent();
    fireEvent.click(screen.getByRole("button", { name: /test connection/i }));

    await waitFor(() => {
      expect(screen.getByText("Connection Successful")).toBeInTheDocument();
    });
    expect(screen.getByText("Target is reachable.")).toBeInTheDocument();
    expect(screen.getByText(/Hello world/)).toBeInTheDocument();
  });

  it("shows error alert with details on failed ping", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          success: false,
          error: "Connection refused",
          suggestion: "Check the URL.",
          available_keys: ["choices", "message"],
          raw_body: '{"choices":[]}',
        }),
    } as Response);

    renderComponent();
    fireEvent.click(screen.getByRole("button", { name: /test connection/i }));

    await waitFor(() => {
      expect(screen.getByText("Connection Failed")).toBeInTheDocument();
    });
    expect(screen.getByText("Check the URL.")).toBeInTheDocument();
    expect(screen.getByText(/Connection refused/)).toBeInTheDocument();
    expect(screen.getByText("choices")).toBeInTheDocument();
    expect(screen.getByText("message")).toBeInTheDocument();
    expect(screen.getByText("Raw response")).toBeInTheDocument();
  });

  it("shows error alert when fetch throws", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("Network failure"));

    renderComponent();
    fireEvent.click(screen.getByRole("button", { name: /test connection/i }));

    await waitFor(() => {
      expect(screen.getByText("Connection Failed")).toBeInTheDocument();
    });
    expect(screen.getByText(/Network failure/)).toBeInTheDocument();
    expect(screen.getByText("Failed to reach the ping endpoint.")).toBeInTheDocument();
  });

  it("sends correct payload to /api/ping", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ success: true, response: "ok", suggestion: "" }),
    } as Response);

    renderComponent({ url: "https://target.com/chat", request_body_template: '{"msg":"{{prompt}}"}' });
    fireEvent.click(screen.getByRole("button", { name: /test connection/i }));

    await waitFor(() => {
      expect(fetchSpy).toHaveBeenCalledWith("/api/ping", expect.objectContaining({
        method: "POST",
        headers: { "Content-Type": "application/json" },
      }));
    });

    const body = JSON.parse(fetchSpy.mock.calls[0]?.[1]?.body as string);
    expect(body.url).toBe("https://target.com/chat");
    expect(body.request_body_template).toBe('{"msg":"{{prompt}}"}');
  });
});
