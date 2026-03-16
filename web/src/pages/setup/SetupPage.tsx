import { useState, useEffect } from "react";
import { Form, Button, Alert, Typography, Space, Modal, Input } from "antd";
import { DownloadOutlined } from "@ant-design/icons";
import { steps } from "../../components/Sidebar";
import YamlHighlight from "../../components/YamlHighlight";
import TargetTypeStep from "./TargetTypeStep";
import TargetConfigStep from "./TargetConfigStep";
import AppContextStep from "./AppContextStep";
import GoalStep from "./GoalStep";
import StrategiesStep from "./StrategiesStep";
import TestConnection from "./TestConnection";
import { configToYaml } from "../../yaml";
import type { Config } from "../../types";

interface SetupPageProps {
  activeStep: string;
  onStepChange: (key: string) => void;
  onConfigPathLoaded: (path: string | null) => void;
}

const stepFields: Record<string, string[][]> = {
  "target-type": [["target", "name"], ["target", "type"]],
  "target-config": [["target", "settings", "url"]],
  "app-context": [],
  "goal": [["goals"]],
  "strategies": [["strategies"]],
  "review": [],
};

function buildGroundTruth(gt: any) {
  const systemPrompt = gt?.system_prompt?.trim() || "";
  const toolsRaw = gt?.tools?.trim() || "";
  const tools = toolsRaw ? toolsRaw.split(",").map((t: string) => t.trim()).filter(Boolean) : [];
  if (!systemPrompt && tools.length === 0) return undefined;
  return { system_prompt: systemPrompt || undefined, tools: tools.length ? tools : undefined };
}

function buildConfig(values: Record<string, any>): Config {
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
        response_selector: settings.response_selector || "response",
        request_body_template:
          settings.request_body_template || '{"message": "{{prompt}}"}',
      },
    },
    goals: values?.goals?.length ? values.goals : ["system_prompt_extraction"],
    strategies: values?.strategies?.length ? values.strategies : ["directly_asking"],
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
  goals: ["system_prompt_extraction"],
  strategies: ["directly_asking"],
};

export default function SetupPage({ activeStep, onStepChange, onConfigPathLoaded }: SetupPageProps) {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [yamlContent, setYamlContent] = useState<string | null>(null);
  const [missingFields, setMissingFields] = useState<string[]>([]);
  const [filenameModalOpen, setFilenameModalOpen] = useState(false);
  const [filename, setFilename] = useState("redteam.yaml");

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
          goals: cfg.goals?.length ? cfg.goals : ["system_prompt_extraction"],
          strategies: cfg.strategies?.length ? cfg.strategies : ["directly_asking"],
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

    const requiredFields = Object.values(stepFields).flat();
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
    const fields = stepFields[activeStep] ?? [];
    if (fields.length > 0) {
      await form.validateFields(fields);
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

  const validateAndDownload = async () => {
    setLoading(true);
    setError(null);
    try {
      const values = await form.validateFields();
      const config = buildConfig(values);
      const res = await fetch("/api/config", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(config),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        const msg = body?.errors?.join("\n") || `Server error: ${res.status}`;
        throw new Error(msg);
      }
      const data: { yaml: string } = await res.json();
      downloadFile(data.yaml, filename);
    } catch (err) {
      setError(err instanceof Error ? err.message : "An unexpected error occurred");
    } finally {
      setLoading(false);
    }
  };

  const handleDownload = () => {
    setFilenameModalOpen(true);
  };

  const confirmDownload = () => {
    setFilenameModalOpen(false);
    validateAndDownload();
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

      <Typography.Title level={4} style={{ marginBottom: 24 }}>
        {steps[currentIndex]?.label}
      </Typography.Title>

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
      <div style={{ display: activeStep === "strategies" ? "block" : "none" }}>
        <StrategiesStep />
      </div>

      {isReview && (
        <>
          {missingFields.length > 0 && (
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
          {yamlContent && <YamlHighlight content={yamlContent} />}
          <div style={{ marginTop: 16, marginBottom: 16 }}>
            <TestConnection />
          </div>
        </>
      )}

      <Space style={{ marginTop: 24 }}>
        {!isFirst && (
          <Button onClick={goBack}>Back</Button>
        )}
        {isReview ? (
          <Button
            type="primary"
            icon={<DownloadOutlined />}
            onClick={handleDownload}
            disabled={missingFields.length > 0}
            loading={loading}
          >
            Download Configuration
          </Button>
        ) : (
          <Button type="primary" onClick={goNext} loading={loading}>
            {currentIndex === steps.length - 2 ? "Review Configuration" : "Next"}
          </Button>
        )}
      </Space>

      <Modal
        title="Save as"
        open={filenameModalOpen}
        onOk={confirmDownload}
        onCancel={() => setFilenameModalOpen(false)}
        okText="Download"
      >
        <Input
          value={filename}
          onChange={(e) => setFilename(e.target.value)}
          onPressEnter={confirmDownload}
        />
      </Modal>
    </Form>
  );
}
