import { Button, Result } from "antd";
import { useNavigate } from "react-router-dom";

export function NotFoundPage() {
  const navigate = useNavigate();
  return (
    <Result
      status="404"
      title="404"
      subTitle="你访问的页面不存在，可能还没被排进一期 MVP。"
      extra={
        <Button type="primary" onClick={() => navigate("/dashboard")}>
          回到控制台
        </Button>
      }
    />
  );
}
