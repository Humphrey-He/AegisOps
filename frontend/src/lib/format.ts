import dayjs from "dayjs";

export function formatDateTime(value?: string) {
  if (!value) {
    return "--";
  }
  return dayjs(value).format("YYYY-MM-DD HH:mm:ss");
}

export function formatRelativeTime(value?: string) {
  if (!value) {
    return "--";
  }
  return dayjs(value).format("MM-DD HH:mm");
}

export function formatBytes(value?: number) {
  if (!value || value <= 0) {
    return "--";
  }

  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = value;
  let index = 0;
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024;
    index += 1;
  }
  const digits = size >= 10 || index === 0 ? 0 : 1;
  return `${size.toFixed(digits)} ${units[index]}`;
}
