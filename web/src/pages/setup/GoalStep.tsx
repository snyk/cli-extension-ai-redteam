import { useEffect, useState } from "react";
import { Form, Checkbox, Space, Spin, Alert } from "antd";

interface EnumEntry {
  value: string;
  description: string;
  display_order: number;
}

export default function GoalStep() {
  const [goals, setGoals] = useState<EnumEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetch("/api/goals")
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      })
      .then((data: EnumEntry[]) => {
        setGoals(data);
        setLoading(false);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load goals");
        setLoading(false);
      });
  }, []);

  if (loading) return <Spin />;
  if (error) return <Alert type="error" message="Failed to load goals" description={error} />;

  return (
    <Form.Item
      name="goals"
      rules={[{ required: true, message: "Please select at least one goal" }]}
    >
      <Checkbox.Group>
        <Space direction="vertical" size="middle">
          {goals.map((g) => (
            <Checkbox key={g.value} value={g.value}>
              <strong>{g.value}</strong>
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
