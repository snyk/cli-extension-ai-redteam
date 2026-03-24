import type { Config } from "./types";

export function configToYaml(config: Config): string {
  const lines: string[] = [];

  lines.push("target:");
  lines.push(`  name: ${quote(config.target.name)}`);
  lines.push(`  type: ${config.target.type}`);

  const ctx = config.target.context;
  const gt = ctx.ground_truth;
  const hasContext = ctx.purpose || gt?.system_prompt || (gt?.tools && gt.tools.length > 0);
  if (hasContext) {
    lines.push("  context:");
    if (ctx.purpose) {
      lines.push(`    purpose: ${quote(ctx.purpose)}`);
    }
    if (gt?.system_prompt || (gt?.tools && gt.tools.length > 0)) {
      lines.push("    ground_truth:");
      if (gt.system_prompt) {
        lines.push(`      system_prompt: ${quote(gt.system_prompt)}`);
      }
      if (gt.tools && gt.tools.length > 0) {
        lines.push("      tools:");
        for (const t of gt.tools) {
          lines.push(`        - ${quote(t)}`);
        }
      }
    }
  }

  lines.push("  settings:");
  lines.push(`    url: ${quote(config.target.settings.url)}`);
  lines.push(`    request_body_template: ${quote(config.target.settings.request_body_template)}`);
  lines.push(`    response_selector: ${quote(config.target.settings.response_selector)}`);
  if (
    typeof config.target.settings.timeout === "number" &&
    config.target.settings.timeout > 0
  ) {
    lines.push(`    timeout: ${config.target.settings.timeout}`);
  }

  const headers = config.target.settings.headers;
  if (headers && headers.length > 0) {
    lines.push("    headers:");
    for (const h of headers) {
      lines.push(`      - name: ${quote(h.name)}`);
      lines.push(`        value: ${quote(h.value)}`);
    }
  }

  if (config.attacks && config.attacks.length > 0) {
    lines.push("attacks:");
    for (const a of config.attacks) {
      lines.push(`  - goal: ${a.goal}`);
      if (a.strategy) {
        lines.push(`    strategy: ${a.strategy}`);
      }
    }
  }

  return lines.join("\n") + "\n";
}

function quote(value: string): string {
  if (!value) return '""';
  if (/[:#{}[\],&*?|>!'"%@`]/.test(value) || value.includes("\n")) {
    if (!value.includes("'")) {
      return `'${value}'`;
    }
    return `"${value.replace(/\\/g, "\\\\").replace(/"/g, '\\"')}"`;
  }
  return value;
}
