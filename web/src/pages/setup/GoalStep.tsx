import { useEffect, useState } from "react";
import { Form, Checkbox, Space, Spin, Alert, Row, Col } from "antd";

interface EnumEntry {
  value: string;
  description: string;
  display_order: number;
}

interface ProfileEntry {
  goal: string;
  strategy?: string;
}

interface Profile {
  id: string;
  name: string;
  description: string;
  entries: ProfileEntry[];
}

export default function GoalStep() {
  const [goals, setGoals] = useState<EnumEntry[]>([]);
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [selectedProfile, setSelectedProfile] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const form = Form.useFormInstance();

  useEffect(() => {
    Promise.all([
      fetch("/api/goals").then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      }),
      fetch("/api/profiles").then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      }),
    ])
      .then(([goalsData, profilesData]: [EnumEntry[], Profile[]]) => {
        setGoals(goalsData);
        setProfiles(profilesData);
        setLoading(false);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load goals");
        setLoading(false);
      });
  }, []);

  const handleProfileClick = (profile: Profile) => {
    const isDeselecting = selectedProfile === profile.id;
    if (isDeselecting) {
      setSelectedProfile(null);
      form.setFieldsValue({ goals: [], attacks: undefined });
      return;
    }
    setSelectedProfile(profile.id);
    const profileGoals = [...new Set(profile.entries.map((e) => e.goal))];
    form.setFieldsValue({ goals: profileGoals, attacks: profile.entries });
  };

  const handleGoalsChange = () => {
    if (selectedProfile) {
      setSelectedProfile(null);
      form.setFieldsValue({ attacks: undefined });
    }
  };

  if (loading) return <Spin />;
  if (error) return <Alert type="error" message="Failed to load goals" description={error} />;

  return (
    <>
      {profiles.length > 0 && (
        <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
          {profiles.map((p) => (
            <Col key={p.id} xs={24} sm={8}>
              {/* Custom div instead of antd Card — Card's CSS-in-JS overrides background styles */}
              <div
                onClick={() => handleProfileClick(p)}
                className={selectedProfile === p.id ? "profile-card profile-card-selected" : "profile-card"}
                data-testid={`profile-card-${p.id}`}
              >
                <div style={{ fontWeight: 600, marginBottom: 4 }}>{p.name}</div>
                <div style={{ color: "var(--pcl-color-fg-secondary)", fontSize: 13 }}>
                  {p.description}
                </div>
              </div>
            </Col>
          ))}
        </Row>
      )}
      <Form.Item name="attacks" hidden><input type="hidden" /></Form.Item>
      <Form.Item
        name="goals"
        rules={[{ required: true, message: "Please select at least one goal" }]}
      >
        <Checkbox.Group onChange={handleGoalsChange}>
          <Space direction="vertical" size="middle">
            {goals.map((g) => (
              <Checkbox key={g.value} value={g.value}>
                <strong>{g.value.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase())}</strong>
                <br />
                <span style={{ color: "var(--pcl-color-fg-secondary)", fontSize: 13 }}>
                  {g.description}
                </span>
              </Checkbox>
            ))}
          </Space>
        </Checkbox.Group>
      </Form.Item>
    </>
  );
}
