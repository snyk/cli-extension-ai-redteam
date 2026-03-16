import { Form, Input } from "antd";

const { TextArea } = Input;

export default function AppContextStep() {
  return (
    <Form.Item
      label="Purpose"
      name={["target", "context", "purpose"]}
      tooltip="Describe what this application does so the red team can craft more targeted probes"
    >
      <TextArea rows={4} placeholder="e.g. Customer support chatbot that helps users with billing inquiries" />
    </Form.Item>
  );
}
