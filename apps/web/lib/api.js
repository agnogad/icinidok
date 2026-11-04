const envBase = process.env.NEXT_PUBLIC_API_BASE;

function defaultBase() {
  if (envBase !== undefined && envBase !== "") return envBase;
  if (typeof window !== "undefined" && window.location.port === "3000") {
    return `http://${window.location.hostname}:8080`;
  }
  return "";
}

export const API_BASE = defaultBase();

export async function api(path, options = {}) {
  const { body, headers, ...rest } = options;
  const response = await fetch(`${API_BASE}${path}`, {
    credentials: "include",
    ...rest,
    headers: {
      ...(body ? { "Content-Type": "application/json" } : {}),
      ...(headers || {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  });

  const text = await response.text();
  let data = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = null;
  }

  if (!response.ok) {
    const error = new Error(
      (data && data.error) || "Bir şeyler ters gitti. Tekrar dene."
    );
    error.status = response.status;
    throw error;
  }
  return data;
}
