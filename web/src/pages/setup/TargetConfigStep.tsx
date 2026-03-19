import { Form, Input, Typography } from "antd";
import HeadersEditor from "./HeadersEditor";
import TestConnection from "./TestConnection";

const { TextArea } = Input;

export function validateHttpUrl(_: unknown, value: string) {
  if (!value) return Promise.reject(new Error("URL is required"));
  try {
    const url = new URL(value);
    if (url.protocol !== "http:" && url.protocol !== "https:") {
      return Promise.reject(new Error("URL must use HTTP or HTTPS protocol"));
    }
    if (!url.host) {
      return Promise.reject(new Error("URL must have a valid host"));
    }
    return Promise.resolve();
  } catch {
    return Promise.reject(new Error("Must be a valid URL"));
  }
}

export function validateRequestBodyTemplate(_: unknown, value: string | undefined) {
  if (!value) return Promise.reject(new Error("Request body template is required"));
  if (!value.includes("{{prompt}}")) {
    return Promise.reject(new Error("Must contain the {{prompt}} placeholder"));
  }
  const replaced = value.replaceAll("{{prompt}}", "test");
  try {
    JSON.parse(replaced);
  } catch {
    return Promise.reject(new Error("Must be valid JSON"));
  }
  return Promise.resolve();
}

export default function TargetConfigStep() {
  return (
    <>
      <Form.Item
        label="Target URL"
        name={["target", "settings", "url"]}
        rules={[{ required: true, validator: validateHttpUrl }]}
        tooltip="The HTTP(S) endpoint that Agent Red Teaming will send prompts to"
      >
        <Input placeholder="https://api.example.com/chat/completions" />
      </Form.Item>

      <Form.Item
        label="Request Body Template"
        name={["target", "settings", "request_body_template"]}
        rules={[{ required: true, validator: validateRequestBodyTemplate }]}
        tooltip="JSON template sent to the target. Use {{prompt}} as a placeholder - Agent Red Teaming replaces it with each probe"
        extra={
          <Typography.Text type="secondary" style={{ fontFamily: "var(--pcl-font-family-mono)", fontSize: 12 }}>
            Example: {`{"message": "{{prompt}}"}`}
          </Typography.Text>
        }
      >
        <TextArea
          rows={3}
          style={{ fontFamily: "var(--pcl-font-family-mono)" }}
        />
      </Form.Item>

      <Form.Item
        label="Response Selector"
        name={["target", "settings", "response_selector"]}
        tooltip={<>JMESPath expression to extract the response from a JSON body (e.g. <code>data.choices[0].message.content</code>). Leave empty to use the raw response body as-is (plain text targets). <a href="https://jmespath.org/specification.html" target="_blank" rel="noopener noreferrer">JMESPath reference</a></>}
      >
        <Input style={{ fontFamily: "var(--pcl-font-family-mono)" }} />
      </Form.Item>

      <Form.Item label="Headers" tooltip="Custom HTTP headers sent with every request (e.g. Authorization, API keys)">
        <HeadersEditor />
      </Form.Item>

      <TestConnection />
    </>
  );
}
