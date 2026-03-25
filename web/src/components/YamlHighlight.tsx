function highlightYaml(yaml: string): string {
  return yaml
    .split("\n")
    .map((line) => {
      // Comments
      if (/^\s*#/.test(line)) {
        return `<span class="yaml-comment">${escape(line)}</span>`;
      }

      // Key: value lines
      const match = line.match(/^(\s*)(- )?([^:]+?)(:\s*)(.*)/);
      if (match) {
        const indent = match[1] ?? "";
        const dash = match[2] ?? "";
        const key = match[3] ?? "";
        const colon = match[4] ?? "";
        const value = match[5] ?? "";
        const parts = [escape(indent)];
        if (dash) parts.push(`<span class="yaml-dash">${escape(dash)}</span>`);
        parts.push(`<span class="yaml-key">${escape(key)}</span>`);
        parts.push(`<span class="yaml-colon">${escape(colon)}</span>`);
        if (value) parts.push(highlightValue(value));
        return parts.join("");
      }

      // List items without keys
      const listMatch = line.match(/^(\s*)(- )(.*)/);
      if (listMatch) {
        const indent = listMatch[1] ?? "";
        const dash = listMatch[2] ?? "";
        const value = listMatch[3] ?? "";
        return `${escape(indent)}<span class="yaml-dash">${escape(dash)}</span>${highlightValue(value)}`;
      }

      return escape(line);
    })
    .join("\n");
}

function highlightValue(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) return "";

  // Quoted strings
  if (/^(['"]).*\1$/.test(trimmed)) {
    return `<span class="yaml-string">${escape(value)}</span>`;
  }
  // Booleans
  if (/^(true|false)$/i.test(trimmed)) {
    return `<span class="yaml-bool">${escape(value)}</span>`;
  }
  // Numbers
  if (/^-?\d+(\.\d+)?$/.test(trimmed)) {
    return `<span class="yaml-number">${escape(value)}</span>`;
  }
  // null
  if (/^(null|~)$/i.test(trimmed)) {
    return `<span class="yaml-null">${escape(value)}</span>`;
  }
  return `<span class="yaml-string">${escape(value)}</span>`;
}

function escape(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

import { useState } from "react";
import { Tooltip } from "antd";
import { CopyOutlined, CheckOutlined } from "@ant-design/icons";

interface YamlHighlightProps {
  content: string;
}

export default function YamlHighlight({ content }: YamlHighlightProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(content);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div style={{ position: "relative" }}>
      <Tooltip title={copied ? "Copied!" : "Copy to clipboard"}>
        <button
          onClick={handleCopy}
          style={{
            position: "absolute",
            top: 8,
            right: 8,
            background: "none",
            border: "1px solid var(--pcl-color-border)",
            borderRadius: 4,
            padding: "4px 6px",
            cursor: "pointer",
            color: "var(--pcl-color-fg-secondary)",
            fontSize: 14,
            lineHeight: 1,
            zIndex: 1,
          }}
        >
          {copied ? <CheckOutlined /> : <CopyOutlined />}
        </button>
      </Tooltip>
      <pre
        className="yaml-highlight"
        dangerouslySetInnerHTML={{ __html: highlightYaml(content) }}
      />
    </div>
  );
}
