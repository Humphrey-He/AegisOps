import { Button, Result, Space, Typography } from "antd";
import { createBrowserRouter } from "react-router-dom";

import { useSessionStore } from "../store/sessionStore";

function HomePage() {
  const token = useSessionStore((state) => state.token);
  const clearSession = useSessionStore((state) => state.clearSession);

  return (
    <div className="fullscreen-center">
      <Result
        status="info"
        title="AegisOps console scaffold is ready"
        subTitle="The frontend foundation is in place and can now be extended with real pages and API wiring."
        extra={
          <Space>
            <Typography.Text type="secondary">
              {token ? "Cached session found" : "No active session"}
            </Typography.Text>
            <Button onClick={clearSession}>Clear session</Button>
          </Space>
        }
      />
    </div>
  );
}

export const router = createBrowserRouter([
  {
    path: "/",
    element: <HomePage />,
  },
]);
