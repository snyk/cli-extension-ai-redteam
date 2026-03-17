import { describe, it, expect } from "vitest";
import { validateHttpUrl, validateRequestBodyTemplate } from "./TargetConfigStep";
import { isValidHttpUrl } from "./TestConnection";

describe("validateHttpUrl", () => {
  it("rejects empty value", async () => {
    await expect(validateHttpUrl(null, "")).rejects.toThrow("URL is required");
  });

  it("rejects non-URL string", async () => {
    await expect(validateHttpUrl(null, "not-a-url")).rejects.toThrow("Must be a valid URL");
  });

  it("rejects ftp:// protocol", async () => {
    await expect(validateHttpUrl(null, "ftp://example.com")).rejects.toThrow("HTTP or HTTPS");
  });

  it("accepts http:// URL", async () => {
    await expect(validateHttpUrl(null, "http://example.com")).resolves.toBeUndefined();
  });

  it("accepts https:// URL", async () => {
    await expect(validateHttpUrl(null, "https://api.example.com/v1")).resolves.toBeUndefined();
  });
});

describe("validateRequestBodyTemplate", () => {
  it("rejects empty value", async () => {
    await expect(validateRequestBodyTemplate(null, "")).rejects.toThrow("required");
  });

  it("rejects undefined value", async () => {
    await expect(validateRequestBodyTemplate(null, undefined)).rejects.toThrow("required");
  });

  it("rejects template missing {{prompt}}", async () => {
    await expect(validateRequestBodyTemplate(null, '{"msg": "hello"}')).rejects.toThrow("{{prompt}}");
  });

  it("rejects invalid JSON", async () => {
    await expect(validateRequestBodyTemplate(null, "{{prompt}} not json")).rejects.toThrow("valid JSON");
  });

  it("accepts valid JSON with {{prompt}} placeholder", async () => {
    await expect(
      validateRequestBodyTemplate(null, '{"message": "{{prompt}}"}'),
    ).resolves.toBeUndefined();
  });
});

describe("isValidHttpUrl", () => {
  it("returns false for undefined", () => {
    expect(isValidHttpUrl(undefined)).toBe(false);
  });

  it("returns false for empty string", () => {
    expect(isValidHttpUrl("")).toBe(false);
  });

  it("returns false for ftp:// protocol", () => {
    expect(isValidHttpUrl("ftp://example.com")).toBe(false);
  });

  it("returns true for http:// URL", () => {
    expect(isValidHttpUrl("http://localhost:8080")).toBe(true);
  });

  it("returns true for https:// URL", () => {
    expect(isValidHttpUrl("https://api.example.com/chat")).toBe(true);
  });
});
