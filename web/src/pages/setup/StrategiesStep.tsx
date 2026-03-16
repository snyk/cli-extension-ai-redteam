import { Form, Checkbox, Space } from "antd";

const strategies = [
  { value: "directly_asking", label: "Directly Asking", description: "Ask directly for the information" },
  { value: "role_play", label: "Role Play", description: "Use role play scenarios" },
];

export default function StrategiesStep() {
  return (
    <Form.Item
      name="strategies"
      rules={[{ required: true, message: "Please select at least one strategy" }]}
    >
      <Checkbox.Group>
        <Space direction="vertical" size="middle">
          {strategies.map((s) => (
            <Checkbox key={s.value} value={s.value}>
              <strong>{s.label}</strong>
              <br />
              <span style={{ color: "var(--pcl-color-fg-secondary)", fontSize: 13 }}>
                {s.description}
              </span>
            </Checkbox>
          ))}
        </Space>
      </Checkbox.Group>
    </Form.Item>
  );
}
