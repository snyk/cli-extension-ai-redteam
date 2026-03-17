import { Form, Input, Select } from "antd";

export default function TargetTypeStep() {
  return (
    <>
      <Form.Item
        label="Target Name"
        name={["target", "name"]}
        rules={[{ required: true, message: "Target name is required" }]}
        tooltip="A human-readable name for this target, used to identify it in scan results"
      >
        <Input placeholder="My AI Chatbot" />
      </Form.Item>

      <Form.Item
        label="Target Type"
        name={["target", "type"]}
        rules={[{ required: true, message: "Target type is required" }]}
        tooltip="The protocol used to communicate with the target"
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
