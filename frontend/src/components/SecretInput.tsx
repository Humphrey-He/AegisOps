import { Input } from "antd";
import type { InputProps } from "antd";

export function SecretInput(props: InputProps) {
  return <Input.Password visibilityToggle {...props} />;
}
