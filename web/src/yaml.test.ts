import { describe, it, expect } from "vitest";
import { configToYaml } from "./yaml";
import type { Config } from "./types";

function minimalConfig(overrides?: Partial<Config>): Config {
  return {
    target: {
      name: "test-target",
      type: "http",
      ...overrides?.target,
      context: { purpose: "", ...overrides?.target?.context },
      settings: {
        url: "https://example.com/api",
        request_body_template: '{"prompt":"<<PROMPT>>"}',
        response_selector: "$.response",
        ...overrides?.target?.settings,
      },
    },
    goals: overrides?.goals ?? ["harmful_content"],
    attacks: "attacks" in (overrides ?? {}) ? overrides!.attacks : [{ goal: "harmful_content" }],
  };
}

describe("configToYaml", () => {
  it("renders minimal config with correct YAML structure", () => {
    const yaml = configToYaml(minimalConfig());
    expect(yaml).toContain("target:");
    expect(yaml).toContain("  name: test-target");
    expect(yaml).toContain("  type: http");
    expect(yaml).toContain("  settings:");
    expect(yaml).toContain("goals:");
    expect(yaml).toContain("  - harmful_content");
    expect(yaml).toContain("attacks:");
    expect(yaml).toContain("  - goal: harmful_content");
    expect(yaml.endsWith("\n")).toBe(true);
  });

  it("renders attacks with strategy when provided", () => {
    const yaml = configToYaml(
      minimalConfig({
        attacks: [
          { goal: "system_prompt_extraction", strategy: "crescendo" },
          { goal: "pii_extraction" },
        ],
      }),
    );
    expect(yaml).toContain("attacks:");
    expect(yaml).toContain("  - goal: system_prompt_extraction");
    expect(yaml).toContain("    strategy: crescendo");
    expect(yaml).toContain("  - goal: pii_extraction");
  });

  it("omits attacks section when attacks is empty", () => {
    const yaml = configToYaml(minimalConfig({ attacks: [] }));
    expect(yaml).not.toContain("attacks:");
  });

  it("omits attacks section when attacks is undefined", () => {
    const yaml = configToYaml(minimalConfig({ attacks: undefined }));
    expect(yaml).not.toContain("attacks:");
  });

  it("omits context block when purpose is empty and no ground_truth", () => {
    const yaml = configToYaml(minimalConfig());
    expect(yaml).not.toContain("  context:");
  });

  it("renders context block when purpose is set", () => {
    const yaml = configToYaml(
      minimalConfig({ target: { context: { purpose: "A helpful chatbot" } } } as Partial<Config>),
    );
    expect(yaml).toContain("  context:");
    expect(yaml).toContain("    purpose: A helpful chatbot");
  });

  it("renders ground_truth with system_prompt and tools", () => {
    const yaml = configToYaml(
      minimalConfig({
        target: {
          context: {
            purpose: "assistant",
            ground_truth: {
              system_prompt: "You are helpful",
              tools: ["search", "calculator"],
            },
          },
        },
      } as Partial<Config>),
    );
    expect(yaml).toContain("    ground_truth:");
    expect(yaml).toContain("      system_prompt: You are helpful");
    expect(yaml).toContain("      tools:");
    expect(yaml).toContain("        - search");
    expect(yaml).toContain("        - calculator");
  });

  it("renders headers as YAML list", () => {
    const yaml = configToYaml(
      minimalConfig({
        target: {
          settings: {
            url: "https://example.com",
            request_body_template: "{}",
            response_selector: "$",
            headers: [
              { name: "Authorization", value: "Bearer token123" },
              { name: "Content-Type", value: "application/json" },
            ],
          },
        },
      } as Partial<Config>),
    );
    expect(yaml).toContain("    headers:");
    expect(yaml).toContain("      - name: Authorization");
    expect(yaml).toContain("        value: Bearer token123");
    expect(yaml).toContain("      - name: Content-Type");
    expect(yaml).toContain("        value: application/json");
  });

  it("quotes values with special characters", () => {
    const yaml = configToYaml(
      minimalConfig({
        target: {
          context: { purpose: "handle: colons & ampersands" },
        },
      } as Partial<Config>),
    );
    expect(yaml).toContain("purpose: 'handle: colons & ampersands'");
  });

  it("renders empty string values as quoted empty strings", () => {
    const yaml = configToYaml(
      minimalConfig({
        target: {
          settings: {
            url: "",
            request_body_template: "",
            response_selector: "",
          },
        },
      } as Partial<Config>),
    );
    expect(yaml).toContain('    url: ""');
    expect(yaml).toContain('    request_body_template: ""');
    expect(yaml).toContain('    response_selector: ""');
  });

  it("uses double quotes when value contains single quotes", () => {
    const yaml = configToYaml(
      minimalConfig({
        target: {
          context: { purpose: "it's a test: value" },
        },
      } as Partial<Config>),
    );
    expect(yaml).toContain('purpose: "it\'s a test: value"');
  });
});
