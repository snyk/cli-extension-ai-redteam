export interface ConfigHeader {
  name: string;
  value: string;
}

export interface ConfigSettings {
  url: string;
  headers?: ConfigHeader[];
  response_selector: string;
  request_body_template: string;
  socketio_path?: string;
  socketio_namespace?: string;
  socketio_send_event_name?: string;
  socketio_response_event_name?: string;
}

export interface ConfigContext {
  purpose: string;
}

export interface ConfigTarget {
  name: string;
  type: string;
  context: ConfigContext;
  settings: ConfigSettings;
}

export interface Config {
  target: ConfigTarget;
  goals: string[];
  strategies: string[];
}
