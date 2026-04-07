export interface ConfigHeader {
  name: string;
  value: string;
}

export interface SessionEndpoint {
  url: string;
  method?: string;
  headers?: ConfigHeader[];
  request_body?: string;
  response_selector?: string;
}

export interface SessionConfig {
  /** Session strategy: "none", "client", "server", or "endpoint". */
  mode: "none" | "client" | "server" | "endpoint";
  /** For "server" mode: "header:<name>", "body:<jmespath>", or "cookie:<name>". */
  extract_from?: string;
  /** For "endpoint" mode: session creation endpoint. */
  endpoint?: SessionEndpoint;
}

export interface ConfigSettings {
  url: string;
  headers?: ConfigHeader[];
  response_selector: string;
  request_body_template: string;
  /** Per-request HTTP timeout in seconds. Omit or 0 for default (60). */
  timeout?: number;
  /** Session management configuration. */
  session?: SessionConfig;
}

export interface ConfigGroundTruth {
  system_prompt?: string;
  tools?: string[];
}

export interface ConfigContext {
  purpose: string;
  ground_truth?: ConfigGroundTruth;
}

export interface ConfigTarget {
  name: string;
  type: string;
  context: ConfigContext;
  settings: ConfigSettings;
}

export interface Attack {
  goal: string;
  strategy?: string;
}

export interface ConfigScan {
  mode?: string;
}

export interface Config {
  target: ConfigTarget;
  goals: string[];
  attacks?: Attack[];
  scan?: ConfigScan;
}
