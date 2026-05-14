import { Button, Result } from "antd";
import { useNavigate } from "react-router-dom";

export function ForbiddenPage() {
  const navigate = useNavigate();
  return (
    <Result
      status="403"
      title="403"
      subTitle="你当前没有访问这个页面的权限。"
      extra={
        <Button type="primary" onClick={() => navigate("/dashboard")}>
          返回工作台
        </Button>
      }
    />
  );
}
