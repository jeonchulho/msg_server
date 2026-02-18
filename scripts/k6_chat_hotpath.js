import http from "k6/http";
import { check, fail, sleep } from "k6";

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";
const TENANT_ID = __ENV.TENANT_ID || "default";
const EMAIL = __ENV.SMOKE_EMAIL || "admin@example.com";
const PASSWORD = __ENV.SMOKE_PASSWORD || "pass1234";
const SLEEP_MS = Number(__ENV.K6_SLEEP_MS || "200");

const VUS = Number(__ENV.K6_VUS || "200");
const DURATION = __ENV.K6_DURATION || "10m";

export const options = {
  vus: VUS,
  duration: DURATION,
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<500", "p(99)<1000"],
  },
};

function postJSON(path, payload, token) {
  const headers = { "Content-Type": "application/json" };
  if (token) headers.Authorization = `Bearer ${token}`;
  return http.post(`${BASE_URL}${path}`, JSON.stringify(payload), { headers });
}

export function setup() {
  const loginRes = postJSON("/api/v1/auth/login", {
    tenant_id: TENANT_ID,
    email: EMAIL,
    password: PASSWORD,
  });

  const loginOK = check(loginRes, {
    "login status 200": (r) => r.status === 200,
    "login has token": (r) => {
      const body = r.json();
      return body && body.access_token;
    },
  });
  if (!loginOK) fail(`login failed: status=${loginRes.status} body=${loginRes.body}`);

  const accessToken = loginRes.json("access_token");

  const roomRes = postJSON(
    "/api/v1/rooms",
    {
      name: "k6-hotpath-room",
      room_type: "group",
      member_ids: [],
    },
    accessToken
  );

  const roomOK = check(roomRes, {
    "create room status 201": (r) => r.status === 201,
    "create room has id": (r) => {
      const body = r.json();
      return body && body.id;
    },
  });
  if (!roomOK) fail(`room create failed: status=${roomRes.status} body=${roomRes.body}`);

  return {
    token: accessToken,
    roomId: String(roomRes.json("id")),
  };
}

export default function (data) {
  const body = `k6-message-${__VU}-${__ITER}`;
  const res = postJSON(
    `/api/v1/rooms/${encodeURIComponent(data.roomId)}/messages`,
    {
      body,
      file_ids: [],
      emojis: [],
    },
    data.token
  );

  check(res, {
    "message status 201": (r) => r.status === 201,
  });

  if (SLEEP_MS > 0) {
    sleep(SLEEP_MS / 1000);
  }
}
