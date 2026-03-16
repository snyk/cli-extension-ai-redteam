import { Form, Input, Select } from "antd";

export default function TargetTypeStep() {
  return (
    <>
      <Form.Item
        label="Target Name"
        name={["target", "name"]}
        rules={[{ required: true, message: "Target name is required" }]}
      >
        <Input placeholder="My AI Chatbot" />
      </Form.Item>

      <Form.Item
        label="Target Hype"
        name={["target", "type"]}
        rules={[{ required: true, message: "Target type is required" }]}
      >
        <Select
          options={[
            { value: "http", label: "HTTP/HTTPS Endpoint" },
          ]}
        />
      </Form.Item>
    </>
  );
}
