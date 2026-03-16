import { Form, Input } from "antd";

const { TextArea } = Input;

export default function AppContextStep() {
  return (
    <>
      <Form.Item
        label="Purpose"
        name={["target", "context", "purpose"]}
        tooltip="Describe what this application does so the red team can craft more targeted probes"
      >
        <TextArea rows={4} placeholder="e.g. Customer support chatbot that helps users with billing inquiries" />
      </Form.Item>

      <Form.Item
        label="System Prompt"
        name={["target", "context", "ground_truth", "system_prompt"]}
        tooltip="The ground truth system prompt of the target, used to evaluate extraction attacks"
      >
        <TextArea rows={4} placeholder="e.g. You are a helpful customer support agent..." />
      </Form.Item>

      <Form.Item
        label="Tools"
        name={["target", "context", "ground_truth", "tools"]}
        tooltip="Comma-separated list of tool names available to the target"
      >
        <Input placeholder="e.g. search_orders, refund_payment, check_balance" />
      </Form.Item>
    </>
  );
}
