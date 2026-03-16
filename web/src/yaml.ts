import type { Config } from "./types";

export function configToYaml(config: Config): string {
  const lines: string[] = [];

  lines.push("target:");
  lines.push(`  name: ${quote(config.target.name)}`);
  lines.push(`  type: ${config.target.type}`);

  if (config.target.context.purpose) {
    lines.push("  context:");
    lines.push(`    purpose: ${quote(config.target.context.purpose)}`);
  }

  lines.push("  settings:");
  lines.push(`    url: ${quote(config.target.settings.url)}`);
  lines.push(`    request_body_template: ${quote(config.target.settings.request_body_template)}`);
  lines.push(`    response_selector: ${quote(config.target.settings.response_selector)}`);

  const headers = config.target.settings.headers;
  if (headers && headers.length > 0) {
    lines.push("    headers:");
    for (const h of headers) {
      lines.push(`      - name: ${quote(h.name)}`);
      lines.push(`        value: ${quote(h.value)}`);
    }
  }

  lines.push("goals:");
  for (const g of config.goals) {
    lines.push(`  - ${g}`);
  }

  lines.push("strategies:");
  for (const s of config.strategies) {
    lines.push(`  - ${s}`);
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
