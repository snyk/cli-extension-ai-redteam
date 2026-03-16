import { Form, Input } from "antd";
import HeadersEditor from "./HeadersEditor";
import TestConnection from "./TestConnection";

const { TextArea } = Input;

function validateHttpUrl(_: unknown, value: string) {
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

function validateRequestBodyTemplate(_: unknown, value: string | undefined) {
  if (!value) return Promise.resolve();
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
      >
        <Input placeholder="https://api.example.com/chat/completions" />
      </Form.Item>

      <Form.Item
        label="Request Body Template"
        name={["target", "settings", "request_body_template"]}
        rules={[{ validator: validateRequestBodyTemplate }]}
        tooltip='JSON template with {{prompt}} placeholder. Leave empty for default: {"message": "{{prompt}}"}'
      >
        <TextArea
          rows={3}
          placeholder='{"message": "{{prompt}}"}'
          style={{ fontFamily: "var(--pcl-font-family-mono)" }}
        />
      </Form.Item>

      <Form.Item
        label="Response Selector"
        name={["target", "settings", "response_selector"]}
        tooltip="JMESPath expression to extract the response. Leave empty for default: response"
      >
        <Input placeholder="response" style={{ fontFamily: "var(--pcl-font-family-mono)" }} />
      </Form.Item>

      <Form.Item label="Headers">
        <HeadersEditor />
      </Form.Item>

      <TestConnection />
    </>
  );
}
