import { Form, Checkbox, Space } from "antd";

const goals = [
  { value: "system_prompt_extraction", label: "System Prompt Extraction", description: "Extract the system prompt from the target" },
  { value: "harmful_content", label: "Harmful Content", description: "Attempt to generate harmful content" },
];

export default function GoalStep() {
  return (
    <Form.Item
      name="goals"
      rules={[{ required: true, message: "Please select at least one goal" }]}
    >
      <Checkbox.Group>
        <Space direction="vertical" size="middle">
          {goals.map((g) => (
            <Checkbox key={g.value} value={g.value}>
              <strong>{g.label}</strong>
              <br />
              <span style={{ color: "var(--pcl-color-fg-secondary)", fontSize: 13 }}>
                {g.description}
              </span>
            </Checkbox>
          ))}
        </Space>
      </Checkbox.Group>
    </Form.Item>
  );
}
