import { useState, useEffect } from "react";
import { Form, Button, Alert, Typography, Space, Tooltip } from "antd";
import { DownloadOutlined, CopyOutlined, CheckOutlined, SaveOutlined } from "@ant-design/icons";
import { steps } from "../../components/Sidebar";
import YamlHighlight from "../../components/YamlHighlight";
import TargetTypeStep from "./TargetTypeStep";
import TargetConfigStep from "./TargetConfigStep";
import AppContextStep from "./AppContextStep";
import GoalStep from "./GoalStep";
import TestConnection from "./TestConnection";
import { configToYaml } from "../../yaml";
import type { Config } from "../../types";

interface SetupPageProps {
  activeStep: string;
  onStepChange: (key: string) => void;
  onConfigPathLoaded: (path: string | null) => void;
}

// Required fields per step — validated on "Next" and flagged as missing on the review step.
const requiredStepFields: Record<string, string[][]> = {
  "target-type": [["target", "name"], ["target", "type"]],
  "target-config": [["target", "settings", "url"], ["target", "settings", "request_body_template"]],
  "app-context": [],
  "goal": [["goals"]],
  "review": [],
};

export function buildGroundTruth(gt: any) {
  const systemPrompt = gt?.system_prompt?.trim() || "";
  const toolsRaw = gt?.tools?.trim() || "";
  const tools = toolsRaw ? toolsRaw.split(",").map((t: string) => t.trim()).filter(Boolean) : [];
  if (!systemPrompt && tools.length === 0) return undefined;
  return { system_prompt: systemPrompt || undefined, tools: tools.length ? tools : undefined };
}

export function buildConfig(values: Record<string, any>): Config {
  const target = values?.target ?? {};
  const settings = target?.settings ?? {};
  return {
    target: {
      name: target.name || "",
      type: target.type || "http",
      context: {
        purpose: target.context?.purpose || "",
        ground_truth: buildGroundTruth(target.context?.ground_truth),
      },
      settings: {
        url: settings.url || "",
        headers: settings.headers?.filter(
          (h: { name: string; value: string }) => h.name && h.value,
        ),
        response_selector: settings.response_selector || "",
        request_body_template: settings.request_body_template || "",
      },
    },
    goals: [],
    attacks: values?.attacks || (values?.goals || []).map((g: string) => ({ goal: g })),
  };
}

function downloadFile(content: string, filename: string) {
  const blob = new Blob([content], { type: "application/x-yaml" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  setTimeout(() => URL.revokeObjectURL(url), 100);
}

const defaultValues = {
  target: { type: "http" },
};

export default function SetupPage({ activeStep, onStepChange, onConfigPathLoaded }: SetupPageProps) {
  const [form] = Form.useForm();
  const [error, setError] = useState<string | null>(null);
  const [yamlContent, setYamlContent] = useState<string | null>(null);
  const [missingFields, setMissingFields] = useState<string[]>([]);
  const [validationError, setValidationError] = useState<{ step: string; message: string } | null>(null);
  const [configFilename, setConfigFilename] = useState("redteam.yaml");
  const [copied, setCopied] = useState(false);
  const watchedValues = Form.useWatch([], form);

  useEffect(() => {
    if (!validationError) return;
    const fields = requiredStepFields[validationError.step] ?? [];
    for (const path of fields) {
      const val = path.reduce((obj: any, k) => obj?.[k], watchedValues);
      if (!val || (typeof val === "string" && !val.trim()) || (Array.isArray(val) && val.length === 0)) {
        return;
      }
    }
    setValidationError(null);
  }, [watchedValues]);

  useEffect(() => {
    let cancelled = false;

    fetch("/api/config")
      .then((res) => {
        if (!res.ok) return null;
        return res.json();
      })
      .then((data) => {
        if (cancelled || !data) return;
        const cfg = data.config;
        onConfigPathLoaded(data.config_path || null);
        if (data.config_path) setConfigFilename(data.config_path);
        if (!cfg) return;
        form.setFieldsValue({
          target: {
            name: cfg.target?.name,
            type: cfg.target?.type || "http",
            context: {
              purpose: cfg.target?.context?.purpose,
              ground_truth: {
                system_prompt: cfg.target?.context?.ground_truth?.system_prompt,
                tools: cfg.target?.context?.ground_truth?.tools?.join(", "),
              },
            },
            settings: {
              url: cfg.target?.settings?.url,
              headers: cfg.target?.settings?.headers,
              response_selector: cfg.target?.settings?.response_selector,
              request_body_template: cfg.target?.settings?.request_body_template,
            },
          },
          goals: cfg.goals || [],
        });
      })
      .catch(() => {});

    return () => { cancelled = true; };
  }, []);

  const checkMissingFields = () => {
    const allErrors = form.getFieldsError();
    const fieldsWithErrors = allErrors
      .filter((f) => f.errors.length > 0)
      .map((f) => f.name.join("."));

    const requiredFields = Object.values(requiredStepFields).flat();
    const values = form.getFieldsValue(true);
    const missing: string[] = [];

    for (const path of requiredFields) {
      const key = path.join(".");
      const val = path.reduce((obj: any, k) => obj?.[k], values);
      const isEmpty = !val || (typeof val === "string" && !val.trim());
      if (isEmpty || fieldsWithErrors.includes(key)) {
        missing.push(key);
      }
    }

    setMissingFields(missing);
  };

  const currentIndex = steps.findIndex((s) => s.key === activeStep);
  const isFirst = currentIndex === 0;
  const isReview = activeStep === "review";

  useEffect(() => {
    if (activeStep === "review") {
      generateYaml();
      checkMissingFields();
    }
  }, [activeStep]);

  const goNext = async () => {
    setValidationError(null);
    const fields = requiredStepFields[activeStep] ?? [];
    if (fields.length > 0) {
      try {
        await form.validateFields(fields);
      } catch {
        const stepLabels: Record<string, string> = {
          "target-type": "target name",
          "target-config": "target URL and request template",
          "goal": "at least one goal",
        };
        const msg = stepLabels[activeStep] ? `Select ${stepLabels[activeStep]}` : "Missing required fields";
        setValidationError({ step: activeStep, message: msg });
        return;
      }
    }

    const next = steps[currentIndex + 1];
    if (!next) return;

    onStepChange(next.key);
  };

  const goBack = () => {
    if (isReview) {
      setYamlContent(null);
    }
    const prev = steps[currentIndex - 1];
    if (prev) onStepChange(prev.key);
  };

  const generateYaml = () => {
    try {
      const values = form.getFieldsValue(true);
      const config = buildConfig(values);
      setYamlContent(configToYaml(config));
    } catch (err) {
      setYamlContent(null);
      setError(err instanceof Error ? err.message : "Failed to generate YAML preview");
    }
  };

  const handleDownload = () => {
    if (yamlContent) {
      downloadFile(yamlContent, configFilename);
      fetch("/api/download-complete", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ filename: configFilename }),
      }).catch(() => {});
    }
  };

  const [saving, setSaving] = useState(false);
  const [saveMessage, setSaveMessage] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const handleSave = () => {
    if (!yamlContent) return;
    setSaving(true);
    fetch("/api/save", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ filename: configFilename, content: yamlContent }),
    })
      .then((res) => {
        if (!res.ok) throw new Error("Save failed");
        setSaved(true);
        setSaveMessage(`Saved! Close this wizard and run: snyk redteam --experimental --config ${configFilename}`);
        setTimeout(() => setSaved(false), 2000);
      })
      .catch(() => setError("Failed to save configuration"))
      .finally(() => setSaving(false));
  };

  return (
    <Form
      form={form}
      layout="vertical"
      autoComplete="off"
      initialValues={defaultValues}
      onFieldsChange={() => {
        if (activeStep === "review") {
          generateYaml();
          checkMissingFields();
        }
      }}
    >
      {error && (
        <Alert
          type="error"
          message="Configuration Error"
          description={error}
          showIcon
          closable
          style={{ marginBottom: 24 }}
        />
      )}

      <Typography.Title level={4} style={{ marginBottom: 4 }}>
        {steps[currentIndex]?.label}
      </Typography.Title>
      <Typography.Text type="secondary" style={{ display: "block", marginBottom: 24 }}>
        {steps[currentIndex]?.description}
      </Typography.Text>

      {isReview && missingFields.length > 0 && (
        <Alert
          type="warning"
          message="Missing required fields"
          description={
            <ul style={{ margin: 0, paddingLeft: 20 }}>
              {missingFields.map((f) => (
                <li key={f}>{f}</li>
              ))}
            </ul>
          }
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}

      <div style={{ display: activeStep === "target-type" ? "block" : "none" }}>
        <TargetTypeStep />
      </div>
      <div style={{ display: activeStep === "target-config" ? "block" : "none" }}>
        <TargetConfigStep />
      </div>
      <div style={{ display: activeStep === "app-context" ? "block" : "none" }}>
        <AppContextStep />
      </div>
      <div style={{ display: activeStep === "goal" ? "block" : "none" }}>
        <GoalStep />
      </div>

      {isReview && (
        <>
          {yamlContent && <YamlHighlight content={yamlContent} />}
          <div style={{ marginTop: 16, marginBottom: 16 }}>
            <TestConnection />
          </div>
          {saveMessage && (
            <Alert
              type="success"
              message="Configuration Saved"
              description={saveMessage}
              showIcon
              closable
              onClose={() => setSaveMessage(null)}
              style={{ marginTop: 16 }}
            />
          )}
        </>
      )}

      <div style={{
        position: "sticky",
        bottom: -24,
        paddingTop: 16,
        paddingBottom: 24,
        background: "#27272a",
        borderTop: "1px solid var(--pcl-color-border)",
        marginTop: 24,
        marginLeft: -24,
        marginRight: -24,
        marginBottom: -24,
        paddingLeft: 24,
        paddingRight: 24,
        zIndex: 10,
      }}>
        <Space>
          {!isFirst && (
            <Button onClick={goBack}>Back</Button>
          )}
          {isReview ? (
            <>
              <Button
                icon={copied ? <CheckOutlined /> : <CopyOutlined />}
                onClick={() => {
                  if (yamlContent) {
                    navigator.clipboard.writeText(yamlContent);
                    setCopied(true);
                    setTimeout(() => setCopied(false), 2000);
                  }
                }}
              >
                {copied ? "Copied" : "Copy"}
              </Button>
              <Tooltip title={`Download ${configFilename} to your browser`}>
                <Button
                  icon={<DownloadOutlined />}
                  onClick={handleDownload}
                >
                  Download
                </Button>
              </Tooltip>
              <Tooltip title={`Save ${configFilename} to the current directory`}>
                <Button
                  type="primary"
                  icon={saved ? <CheckOutlined /> : <SaveOutlined />}
                  onClick={handleSave}
                  loading={saving}
                >
                  {saved ? "Saved" : "Save"}
                </Button>
              </Tooltip>
            </>
          ) : (
            <Button type="primary" onClick={goNext}>
              {currentIndex === steps.length - 2 ? "Review Configuration" : "Next"}
            </Button>
          )}
          {validationError && validationError.step === activeStep && (
            <span style={{ color: "#ff4d4f", fontSize: 13 }}>{validationError.message}</span>
          )}
        </Space>
      </div>

    </Form>
  );
}
