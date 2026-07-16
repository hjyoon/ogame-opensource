type APIErrorBody = {
  error?: unknown;
  message?: unknown;
  actionIssue?: { message?: unknown } | null;
  issues?: Array<{ message?: unknown }>;
};

export async function readAPIJSON<T>(response: Response, label: string, allowedErrorStatuses: number[] = []): Promise<T> {
  const text = await response.text();
  if (!response.ok && !allowedErrorStatuses.includes(response.status)) {
    throw new Error(apiErrorMessage(text) ?? `${label} returned ${response.status}`);
  }
  if (text.trim() === "") {
    throw new Error(`${label} response was empty`);
  }
  try {
    return JSON.parse(text) as T;
  } catch {
    if (!response.ok) {
      throw new Error(text.trim() || `${label} returned ${response.status}`);
    }
    throw new Error(`${label} returned an invalid JSON response`);
  }
}

function apiErrorMessage(text: string): string | null {
  const trimmed = text.trim();
  if (trimmed === "") {
    return null;
  }
  try {
    const body = JSON.parse(trimmed) as APIErrorBody;
    for (const value of [body.error, body.message, body.actionIssue?.message, body.issues?.[0]?.message]) {
      if (typeof value === "string" && value.trim() !== "") {
        return value.trim();
      }
    }
  } catch {
    return trimmed;
  }
  return null;
}
