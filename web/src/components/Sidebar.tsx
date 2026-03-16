import { AimOutlined, SettingOutlined, FileTextOutlined, EyeOutlined, FlagOutlined, ThunderboltOutlined } from "@ant-design/icons";
import styles from "./Sidebar.module.scss";

export interface Step {
  key: string;
  label: string;
  icon: React.ReactNode;
}

export const steps: Step[] = [
  { key: "target-type", label: "Target Type", icon: <AimOutlined /> },
  { key: "target-config", label: "Target Configuration", icon: <SettingOutlined /> },
  { key: "app-context", label: "Application Context", icon: <FileTextOutlined /> },
  { key: "goal", label: "Goals", icon: <FlagOutlined /> },
  { key: "strategies", label: "Strategies", icon: <ThunderboltOutlined /> },
  { key: "review", label: "Review and Download", icon: <EyeOutlined /> },
];

interface SidebarProps {
  activeStep: string;
  onStepClick: (key: string) => void;
}

export default function Sidebar({ activeStep, onStepClick }: SidebarProps) {
  return (
    <aside className={styles.sidebar}>
      <div className={styles.header}>
        <span className={styles.logo}>Snyk Red Team</span>
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
