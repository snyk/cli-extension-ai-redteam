import { useEffect, useState } from "react";
import { Form, Checkbox, Space, Spin, Alert } from "antd";

interface EnumEntry {
  value: string;
  description: string;
  display_order: number;
}

export default function StrategiesStep() {
  const [strategies, setStrategies] = useState<EnumEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetch("/api/strategies")
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      })
      .then((data: EnumEntry[]) => {
        setStrategies(data);
        setLoading(false);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load strategies");
        setLoading(false);
      });
  }, []);

  if (loading) return <Spin />;
  if (error) return <Alert type="error" message="Failed to load strategies" description={error} />;

  return (
    <Form.Item
      name="strategies"
      rules={[{ required: true, message: "Please select at least one strategy" }]}
    >
      <Checkbox.Group>
        <Space direction="vertical" size="middle">
          {strategies.map((s) => (
            <Checkbox key={s.value} value={s.value}>
              <strong>{s.value}</strong>
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
