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
