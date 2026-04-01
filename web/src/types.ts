export interface ConfigHeader {
  name: string;
  value: string;
}

export interface ConfigSettings {
  url: string;
  headers?: ConfigHeader[];
  response_selector: string;
  request_body_template: string;
  /** Per-request HTTP timeout in seconds. Omit or 0 for default (60). */
  timeout?: number;
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
