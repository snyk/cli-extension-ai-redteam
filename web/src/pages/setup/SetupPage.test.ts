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
    expect(config.goals).toEqual([]);
    expect(config.attacks).toEqual([{ goal: "system_prompt_extraction" }]);
  });

  it("uses profile attacks with strategies when present", () => {
    const values = {
      target: { name: "t", type: "http", settings: {} },
      goals: ["system_prompt_extraction"],
      attacks: [
        { goal: "system_prompt_extraction", strategy: "crescendo" },
        { goal: "pii_extraction", strategy: "role_play" },
      ],
    };
    const config = buildConfig(values);
    expect(config.goals).toEqual([]);
    expect(config.attacks).toEqual([
      { goal: "system_prompt_extraction", strategy: "crescendo" },
      { goal: "pii_extraction", strategy: "role_play" },
    ]);
  });

  it("converts goals to attacks without strategy when no profile", () => {
    const values = {
      target: { name: "t", type: "http", settings: {} },
      goals: ["harmful_content", "pii_extraction"],
    };
    const config = buildConfig(values);
    expect(config.goals).toEqual([]);
    expect(config.attacks).toEqual([
      { goal: "harmful_content" },
      { goal: "pii_extraction" },
    ]);
  });

  it("returns empty attacks when no goals provided", () => {
    const config = buildConfig({ target: { name: "t", type: "http", settings: {} } });
    expect(config.goals).toEqual([]);
    expect(config.attacks).toEqual([]);
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
