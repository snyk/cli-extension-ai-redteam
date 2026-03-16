import { AimOutlined, SettingOutlined, FileTextOutlined, EyeOutlined, FlagOutlined, ThunderboltOutlined } from "@ant-design/icons";
import styles from "./Sidebar.module.scss";

export interface Step {
  key: string;
  label: string;
  description: string;
  icon: React.ReactNode;
}

export const steps: Step[] = [
  { key: "target-type", label: "Target Type", description: "Name your target and choose how it communicates.", icon: <AimOutlined /> },
  { key: "target-config", label: "Target Configuration", description: "Configure the endpoint URL, headers, and request/response format.", icon: <SettingOutlined /> },
  { key: "app-context", label: "Application Context", description: "Describe the application's purpose and provide ground truth for scoring.", icon: <FileTextOutlined /> },
  { key: "goal", label: "Goals", description: "Select what the red team should try to achieve.", icon: <FlagOutlined /> },
  { key: "strategies", label: "Strategies", description: "Choose the attack strategies to use during the scan.", icon: <ThunderboltOutlined /> },
  { key: "review", label: "Review and Download", description: "Review the generated configuration and download it.", icon: <EyeOutlined /> },
];

interface SidebarProps {
  activeStep: string;
  onStepClick: (key: string) => void;
  configPath: string | null;
}

export default function Sidebar({ activeStep, onStepClick, configPath }: SidebarProps) {
  return (
    <aside className={styles.sidebar}>
      <div className={styles.header}>
        <span className={styles.logo}>Agent Red Teaming</span>
        <span className={styles.subtitle}>
          {configPath ? `Configuring ${configPath}` : "New Configuration"}
        </span>
      </div>
      <nav className={styles.nav}>
        {steps.map((step) => (
          <button
            key={step.key}
            className={`${styles.navItem} ${activeStep === step.key ? styles.active : ""}`}
            onClick={() => onStepClick(step.key)}
          >
            <span className={styles.navIcon}>{step.icon}</span>
            <span className={styles.navLabel}>{step.label}</span>
          </button>
        ))}
      </nav>
    </aside>
  );
}
