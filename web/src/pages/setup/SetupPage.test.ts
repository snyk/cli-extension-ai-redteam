import { describe, it, expect } from "vitest";
import { buildGroundTruth, buildConfig } from "./SetupPage";

describe("buildGroundTruth", () => {
  it("returns undefined when both fields are empty", () => {
    expect(buildGroundTruth({})).toBeUndefined();
    expect(buildGroundTruth({ system_prompt: "", tools: "" })).toBeUndefined();
  });

  it("returns system_prompt only when tools is empty", () => {
    const result = buildGroundTruth({ system_prompt: "You are a helpful bot", tools: "" });
    expect(result).toEqual({ system_prompt: "You are a helpful bot", tools: undefined });
  });

  it("returns tools only when system_prompt is empty", () => {
    const result = buildGroundTruth({ system_prompt: "", tools: "search, browse" });
    expect(result).toEqual({ system_prompt: undefined, tools: ["search", "browse"] });
  });

  it("returns both when both are provided", () => {
    const result = buildGroundTruth({ system_prompt: "Be helpful", tools: "calc,search" });
    expect(result).toEqual({ system_prompt: "Be helpful", tools: ["calc", "search"] });
  });

  it("trims whitespace from system_prompt and tool names", () => {
    const result = buildGroundTruth({ system_prompt: "  hello  ", tools: "  a , b , c " });
    expect(result).toEqual({ system_prompt: "hello", tools: ["a", "b", "c"] });
  });

  it("returns undefined for null/undefined input", () => {
    expect(buildGroundTruth(null)).toBeUndefined();
    expect(buildGroundTruth(undefined)).toBeUndefined();
  });
});

describe("buildConfig", () => {
  it("maps form values to Config shape", () => {
    const values = {
      target: {
        name: "My API",
        type: "http",
        context: { purpose: "chatbot" },
        settings: {
          url: "https://api.example.com",
          headers: [{ name: "Authorization", value: "Bearer tok" }],
          response_selector: "data.response",
          request_body_template: '{"prompt": "{{prompt}}"}',
        },
      },
      goals: ["system_prompt_extraction"],
    };

    const config = buildConfig(values);
    expect(config.target.name).toBe("My API");
    expect(config.target.type).toBe("http");
    expect(config.target.context.purpose).toBe("chatbot");
    expect(config.target.settings.url).toBe("https://api.example.com");
    expect(config.target.settings.headers).toHaveLength(1);
    expect(config.goals).toEqual(["system_prompt_extraction"]);
    expect(config.attacks).toEqual([{ goal: "system_prompt_extraction" }]);
  });

  it("applies default goals and attacks when empty", () => {
    const config = buildConfig({ target: { name: "t", type: "http", settings: {} } });
    expect(config.goals).toEqual(["system_prompt_extraction"]);
    expect(config.attacks).toEqual([{ goal: "system_prompt_extraction" }]);
  });

  it("filters out incomplete headers", () => {
    const values = {
      target: {
        settings: {
          headers: [
            { name: "X-Key", value: "val" },
            { name: "", value: "orphan" },
            { name: "NoVal", value: "" },
          ],
        },
      },
    };
    const config = buildConfig(values);
    expect(config.target.settings.headers).toEqual([{ name: "X-Key", value: "val" }]);
  });

  it("handles missing/undefined target gracefully", () => {
    const config = buildConfig({});
    expect(config.target.name).toBe("");
    expect(config.target.type).toBe("http");
    expect(config.target.settings.url).toBe("");
  });
});
