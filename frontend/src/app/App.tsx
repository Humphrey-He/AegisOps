import { RouterProvider } from "react-router-dom";
import { Space, Spin, Typography } from "antd";
import { router } from "./router";
import { useBootstrapSession, useSessionStore } from "../store/sessionStore";

function App() {
  useBootstrapSession();
  const bootstrapped = useSessionStore((state) => state.bootstrapped);

  if (!bootstrapped) {
    return (
      <div className="fullscreen-center">
        <Space direction="vertical" size={12} align="center">
          <Spin size="large" />
          <Typography.Text type="secondary">AegisOps 正在准备控制台...</Typography.Text>
        </Space>
      </div>
    );
  }

  return <RouterProvider router={router} />;
}

export default App;
