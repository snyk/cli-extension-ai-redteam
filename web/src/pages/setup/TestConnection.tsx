import { useState } from "react";
import { Form, Button, Alert } from "antd";

interface PingResult {
  success: boolean;
  response?: string;
  error?: string;
  suggestion: string;
  raw_body?: string;
  available_keys?: string[];
}

export function isValidHttpUrl(value: string | undefined): boolean {
  if (!value) return false;
  try {
    const url = new URL(value);
    return (url.protocol === "http:" || url.protocol === "https:") && !!url.host;
  } catch {
    return false;
  }
}

export default function TestConnection() {
  const form = Form.useFormInstance();
  const urlValue = Form.useWatch(["target", "settings", "url"], form);
  const requestBodyTemplate = Form.useWatch(["target", "settings", "request_body_template"], form);
  const [pinging, setPinging] = useState(false);
  const [pingResult, setPingResult] = useState<PingResult | null>(null);
  const canTest = isValidHttpUrl(urlValue) && !!requestBodyTemplate?.trim();

  const handleTestConnection = async () => {
    setPinging(true);
    setPingResult(null);
    try {
      const settings = form.getFieldValue(["target", "settings"]) ?? {};
      const res = await fetch("/api/ping", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          url: settings.url || "",
          headers:
            settings.headers?.filter(
              (h: { name: string; value: string }) => h?.name && h?.value,
            ) || [],
          request_body_template: settings.request_body_template || "",
          response_selector: settings.response_selector || "",
          timeout: typeof settings.timeout === "number" ? settings.timeout : 0,
        }),
      });
      const data: PingResult = await res.json();
      setPingResult(data);
    } catch (err) {
      setPingResult({
        success: false,
        error: err instanceof Error ? err.message : "Unknown error",
        suggestion: "Failed to reach the ping endpoint.",
      });
    } finally {
      setPinging(false);
    }
  };

  return (
    <>
      <Button onClick={handleTestConnection} loading={pinging} disabled={!canTest}>
        Test Connection
      </Button>

      {pingResult && pingResult.success && (
        <Alert
          type="success"
          message="Connection Successful"
          description={
            <>
              <div>{pingResult.suggestion}</div>
              <div style={{ marginTop: 8, fontFamily: "var(--pcl-font-family-mono)" }}>
                Response: {pingResult.response}
              </div>
            </>
          }
          showIcon
          closable
          style={{ marginTop: 16 }}
        />
      )}

      {pingResult && !pingResult.success && (
        <Alert
          type="error"
          message="Connection Failed"
          description={
            <>
              <div>{pingResult.suggestion}</div>
              {pingResult.error && (
                <div
                  style={{
                    marginTop: 8,
                    fontFamily: "var(--pcl-font-family-mono)",
                    fontSize: 12,
                  }}
                >
                  Error: {pingResult.error}
                </div>
              )}
              {pingResult.available_keys && pingResult.available_keys.length > 0 && (
                <div style={{ marginTop: 8 }}>
                  Available selectors:{" "}
                  {pingResult.available_keys.map((key, i) => (
                    <span key={key}>
                      {i > 0 && ", "}
                      <code
                        style={{
                          background: "rgba(0,0,0,0.06)",
                          padding: "1px 4px",
                          borderRadius: 3,
                          fontSize: 12,
                        }}
                      >
                        {key}
                      </code>
                    </span>
                  ))}
                </div>
              )}
              {pingResult.raw_body && (
                <details style={{ marginTop: 8 }}>
                  <summary style={{ cursor: "pointer" }}>Raw response</summary>
                  <pre
                    style={{
                      fontSize: 12,
                      maxHeight: 120,
                      overflow: "auto",
                      marginTop: 4,
                    }}
                  >
                    {(() => {
                      try {
                        return JSON.stringify(JSON.parse(pingResult.raw_body), null, 2);
                      } catch {
                        return pingResult.raw_body;
                      }
                    })()}
                  </pre>
                </details>
              )}
            </>
          }
          showIcon
          style={{ marginTop: 16 }}
        />
      )}
    </>
  );
}
