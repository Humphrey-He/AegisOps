export type FieldErrors = Record<string, string>;

export type ApiResponse<T> = {
  code: string;
  message: string;
  data: T;
  traceId: string;
};

export class ApiError extends Error {
  status: number;
  code: string;
  traceId: string;
  fieldErrors?: FieldErrors;

  constructor(options: {
    status: number;
    code: string;
    message: string;
    traceId: string;
    fieldErrors?: FieldErrors;
  }) {
    super(options.message);
    this.name = "ApiError";
    this.status = options.status;
    this.code = options.code;
    this.traceId = options.traceId;
    this.fieldErrors = options.fieldErrors;
  }
}
