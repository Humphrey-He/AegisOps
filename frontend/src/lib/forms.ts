import type { FormInstance } from "antd";
import { ApiError } from "../types/api";

export function applyFormErrors(form: FormInstance, error: unknown) {
  if (!(error instanceof ApiError) || !error.fieldErrors) {
    return;
  }
  form.setFields(
    Object.entries(error.fieldErrors).map(([name, errors]) => ({
      name,
      errors: [errors],
    })),
  );
}

export function getErrorMessage(error: unknown, fallback = "请求失败，请稍后再试。") {
  if (error instanceof Error) {
    return error.message;
  }
  return fallback;
}
