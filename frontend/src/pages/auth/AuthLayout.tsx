import {
  AuditOutlined,
  BellOutlined,
  DeploymentUnitOutlined,
  DesktopOutlined,
  KeyOutlined,
  SafetyCertificateOutlined,
} from "@ant-design/icons";
import { Space, Tag, Typography } from "antd";
import { type ReactNode, useEffect, useState } from "react";
import { API_BASE_URL, APP_VERSION, USE_MOCK } from "../../lib/config";

const featureItems = [
  {
    icon: <DesktopOutlined />,
    title: "资产接入",
    description: "统一纳管主机、Docker、Nginx 等运行资源。",
  },
  {
    icon: <DeploymentUnitOutlined />,
    title: "任务追踪",
    description: "所有执行动作进入任务链路，便于持续追踪。",
  },
  {
    icon: <BellOutlined />,
    title: "告警闭环",
    description: "异常事件可确认、处理并关联回滚建议。",
  },
  {
    icon: <AuditOutlined />,
    title: "操作审计",
    description: "关键变更自动留下可追溯的审计记录。",
  },
];

type AuthLayoutProps = {
  children: ReactNode;
  panelTitle?: string;
  panelDescription?: string;
};

type ApiReachability = "checking" | "ready" | "unavailable";

function formatVersionLabel(version: string) {
  return version.startsWith("v") ? version : `v${version}`;
}

export function AuthLayout({
  children,
  panelTitle = "企业级运维控制台",
  panelDescription = "统一管理资产接入、任务执行、告警响应与操作审计，让运维动作有上下文、有记录、可追踪。",
}: AuthLayoutProps) {
  const [apiReachability, setApiReachability] = useState<ApiReachability>(USE_MOCK ? "ready" : "checking");
  const isLocalEnvironment =
    typeof window !== "undefined" &&
    ["localhost", "127.0.0.1", "::1"].includes(window.location.hostname.toLowerCase());

  useEffect(() => {
    if (USE_MOCK) {
      return;
    }

    const controller = new AbortController();

    async function probeApiReadiness() {
      try {
        const response = await fetch(`${API_BASE_URL}/auth/me`, {
          method: "GET",
          signal: controller.signal,
        });

        if (response.ok || response.status === 401 || response.status === 403) {
          setApiReachability("ready");
          return;
        }

        setApiReachability("unavailable");
      } catch (error) {
        if (!controller.signal.aborted) {
          setApiReachability("unavailable");
        }
      }
    }

    void probeApiReadiness();

    return () => {
      controller.abort();
    };
  }, []);

  const environmentLabel = USE_MOCK ? "Demo" : isLocalEnvironment ? "Local" : "Production";
  const dataSourceLabel = USE_MOCK
    ? "Mock 数据源"
    : apiReachability === "checking"
      ? isLocalEnvironment
        ? "Local API 检测中"
        : "API 检测中"
      : apiReachability === "ready"
        ? isLocalEnvironment
          ? "Local API 已就绪"
          : "API 已就绪"
        : isLocalEnvironment
          ? "Local API 待联调"
          : "API 待联调";
  const environmentColor = USE_MOCK ? "gold" : isLocalEnvironment ? "blue" : "green";
  const versionLabel = formatVersionLabel(APP_VERSION);

  return (
    <main className="auth-shell">
      <div className="auth-topbar" aria-label="AegisOps 入口信息">
        <Space size={12} align="center">
          <span className="auth-brand-mark" aria-hidden>
            <SafetyCertificateOutlined />
          </span>
          <Space direction="vertical" size={0}>
            <Typography.Text strong>AegisOps</Typography.Text>
            <Typography.Text type="secondary" className="auth-brand-caption">
              Enterprise Operations Console
            </Typography.Text>
          </Space>
        </Space>
        <Space size={8} wrap>
          <Tag color={environmentColor}>{environmentLabel}</Tag>
          <Tag>{dataSourceLabel}</Tag>
          <Tag>{versionLabel}</Tag>
        </Space>
      </div>

      <div className="auth-layout">
        <section className="auth-brand-panel" aria-labelledby="auth-product-title">
          <div>
            <div className="auth-kicker">
              <KeyOutlined />
              <span>受控访问 · 审计留痕 · 权限治理</span>
            </div>
            <Typography.Title id="auth-product-title" level={1} className="auth-product-title">
              {panelTitle}
            </Typography.Title>
            <Typography.Paragraph className="auth-product-description">{panelDescription}</Typography.Paragraph>
          </div>

          <div className="auth-feature-grid">
            {featureItems.map((item) => (
              <div key={item.title} className="auth-feature-item">
                <span className="auth-feature-icon" aria-hidden>
                  {item.icon}
                </span>
                <div>
                  <Typography.Text strong>{item.title}</Typography.Text>
                  <Typography.Text type="secondary" className="auth-feature-description">
                    {item.description}
                  </Typography.Text>
                </div>
              </div>
            ))}
          </div>

          <div className="auth-status-row">
            <div className="auth-status-item">
              <span className="auth-status-dot" aria-hidden />
              <span>{dataSourceLabel}</span>
            </div>
            <div className="auth-status-item">
              <span className="auth-status-dot auth-status-dot--secure" aria-hidden />
              <span>RBAC 已启用</span>
            </div>
            <div className="auth-status-item">
              <span className="auth-status-dot auth-status-dot--audit" aria-hidden />
              <span>安全审计已启用</span>
            </div>
          </div>
        </section>

        <section className="auth-form-panel" aria-label="认证表单">
          {children}
        </section>
      </div>

      <footer className="auth-page-footer">
        <span>AegisOps {versionLabel}</span>
        <span>内部运维系统</span>
        <span>请使用授权账号访问</span>
      </footer>
    </main>
  );
}
