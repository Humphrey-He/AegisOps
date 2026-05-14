import { RouterProvider } from "react-router-dom";
import { Spin } from "antd";
import { router } from "./router";
import { useBootstrapSession, useSessionStore } from "../store/sessionStore";

function App() {
  useBootstrapSession();
  const bootstrapped = useSessionStore((state) => state.bootstrapped);

  if (!bootstrapped) {
    return (
      <div className="fullscreen-center">
        <Spin size="large" tip="AegisOps 正在准备控制台..." />
      </div>
    );
  }

  return <RouterProvider router={router} />;
}

export default App;
