import React from "react";
import ReactDOM from "react-dom/client";
import { ConfigProvider } from "antd";
import "./pcl-variables.scss";
import "./global.scss";
import { getThemeConfig } from "./theme";
import App from "./App";

document.documentElement.setAttribute("data-theme", "dark");

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ConfigProvider theme={getThemeConfig("dark")}>
      <App />
    </ConfigProvider>
  </React.StrictMode>,
);
