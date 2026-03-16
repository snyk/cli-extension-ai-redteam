import { useState } from "react";
import Sidebar from "./components/Sidebar";
import SetupPage from "./pages/setup/SetupPage";

export default function App() {
  const [activeStep, setActiveStep] = useState("target-type");

  return (
    <div style={{ display: "flex", minHeight: "100vh" }}>
      <Sidebar activeStep={activeStep} onStepClick={setActiveStep} />
      <main
        style={{
          flex: 1,
          background: "var(--pcl-color-bg-surface)",
          padding: 24,
          overflow: "auto",
        }}
      >
        <SetupPage activeStep={activeStep} onStepChange={setActiveStep} />
      </main>
    </div>
  );
}
