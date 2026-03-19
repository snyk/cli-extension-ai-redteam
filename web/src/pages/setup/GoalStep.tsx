import { useEffect, useState } from "react";
import { Form, Checkbox, Space, Spin, Alert, Row, Col, Tag, Tooltip } from "antd";

interface EnumEntry {
  value: string;
  description: string;
  display_order: number;
  strategies?: string[];
}

interface StrategyEntry {
  value: string;
  description: string;
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
  const [strategyDescriptions, setStrategyDescriptions] = useState<Map<string, string>>(new Map());
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [selectedProfile, setSelectedProfile] = useState<string | null>(null);
  const [selectedStrategies, setSelectedStrategies] = useState<Map<string, Set<string>>>(new Map());
  const [checkedGoals, setCheckedGoals] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const form = Form.useFormInstance();

  useEffect(() => {
    Promise.all([
      fetch("/api/goals").then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      }),
      fetch("/api/strategies").then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      }),
      fetch("/api/profiles").then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      }),
    ])
      .then(([goalsData, strategiesData, profilesData]: [EnumEntry[], StrategyEntry[], Profile[]]) => {
        setGoals(goalsData);
        setStrategyDescriptions(new Map(strategiesData.map((s) => [s.value, s.description])));
        setProfiles(profilesData);
        setLoading(false);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load goals");
        setLoading(false);
      });
  }, []);

  // Sync attacks form field whenever selectedStrategies or checkedGoals change
  useEffect(() => {
    const attacks: { goal: string; strategy?: string }[] = [];
    for (const goal of checkedGoals) {
      const strategies = selectedStrategies.get(goal);
      if (strategies && strategies.size > 0) {
        for (const strategy of strategies) {
          attacks.push({ goal, strategy });
        }
      } else {
        attacks.push({ goal });
      }
    }
    form.setFieldsValue({ attacks: attacks.length > 0 ? attacks : undefined });
  }, [selectedStrategies, checkedGoals, form]);

  const handleProfileClick = (profile: Profile) => {
    const isDeselecting = selectedProfile === profile.id;
    if (isDeselecting) {
      setSelectedProfile(null);
      setCheckedGoals([]);
      setSelectedStrategies(new Map());
      form.setFieldsValue({ goals: [], attacks: undefined });
      return;
    }
    setSelectedProfile(profile.id);
    const profileGoals = [...new Set(profile.entries.map((e) => e.goal))];
    setCheckedGoals(profileGoals);

    // Select only profile-specified strategies
    const newStrategies = new Map<string, Set<string>>();
    for (const entry of profile.entries) {
      if (entry.strategy) {
        if (!newStrategies.has(entry.goal)) newStrategies.set(entry.goal, new Set());
        newStrategies.get(entry.goal)!.add(entry.strategy);
      }
    }
    setSelectedStrategies(newStrategies);
    form.setFieldsValue({ goals: profileGoals });
  };

  const handleGoalsChange = (newCheckedGoals: string[]) => {
    const prev = new Set(checkedGoals);
    const next = new Set(newCheckedGoals);

    const newStrategies = new Map(selectedStrategies);

    // Newly checked goals: pre-select ALL strategies
    for (const goal of newCheckedGoals) {
      if (!prev.has(goal)) {
        const goalEntry = goals.find((g) => g.value === goal);
        if (goalEntry?.strategies && goalEntry.strategies.length > 0) {
          newStrategies.set(goal, new Set(goalEntry.strategies));
        }
      }
    }

    // Unchecked goals: remove strategies
    for (const goal of checkedGoals) {
      if (!next.has(goal)) {
        newStrategies.delete(goal);
      }
    }

    setCheckedGoals(newCheckedGoals);
    setSelectedStrategies(newStrategies);

    if (selectedProfile) {
      setSelectedProfile(null);
    }
  };

  const handleStrategyToggle = (goal: string, strategy: string, checked: boolean) => {
    const newStrategies = new Map(selectedStrategies);
    const goalStrategies = new Set(newStrategies.get(goal) || []);

    if (checked) {
      goalStrategies.add(strategy);
    } else {
      goalStrategies.delete(strategy);
    }

    let newChecked = checkedGoals;
    if (goalStrategies.size > 0) {
      newStrategies.set(goal, goalStrategies);
      if (!checkedGoals.includes(goal)) {
        newChecked = [...checkedGoals, goal];
      }
    } else {
      newStrategies.delete(goal);
      newChecked = checkedGoals.filter((g) => g !== goal);
    }

    setSelectedStrategies(newStrategies);
    setCheckedGoals(newChecked);
    form.setFieldsValue({ goals: newChecked });

    if (selectedProfile) {
      setSelectedProfile(null);
    }
  };

  if (loading) return <Spin />;
  if (error) return <Alert type="error" message="Failed to load goals" description={error} />;

  return (
    <>
      {profiles.length > 0 && (
        <>
        <div style={{ marginBottom: 12, fontWeight: 600 }}>Profiles</div>
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
        </>
      )}
      <Form.Item name="attacks" hidden><input type="hidden" /></Form.Item>
      <Form.Item
        name="goals"
        rules={[{ required: true, message: "Please select at least one goal" }]}
      >
        <Checkbox.Group onChange={(vals) => handleGoalsChange(vals as string[])}>
          <Space direction="vertical" size="middle">
            {goals.map((g) => (
              <Checkbox key={g.value} value={g.value}>
                <span style={{ userSelect: "none" }}>
                  <strong>{g.value.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase())}</strong>
                  {g.strategies && g.strategies.length > 0 && (
                    <span onClick={(e) => { e.stopPropagation(); e.preventDefault(); }} onMouseDown={(e) => e.preventDefault()} style={{ marginLeft: 8 }}>
                      {g.strategies.map((s) => {
                        const isChecked = selectedStrategies.get(g.value)?.has(s) ?? false;
                        const desc = strategyDescriptions.get(s);
                        const tag = (
                          <Tag.CheckableTag
                            key={s}
                            checked={isChecked}
                            onChange={(checked) => handleStrategyToggle(g.value, s, checked)}
                            style={{
                              fontSize: 11,
                              lineHeight: "18px",
                              ...(!isChecked ? { border: "1px solid #434343", color: "#8c8c8c" } : {}),
                            }}
                          >
                            {s.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase())}
                          </Tag.CheckableTag>
                        );
                        return desc ? <Tooltip key={s} title={desc}>{tag}</Tooltip> : tag;
                      })}
                    </span>
                  )}
                  <br />
                  <span style={{ color: "var(--pcl-color-fg-secondary)", fontSize: 13 }}>
                    {g.description}
                  </span>
                </span>
              </Checkbox>
            ))}
          </Space>
        </Checkbox.Group>
      </Form.Item>
    </>
  );
}
