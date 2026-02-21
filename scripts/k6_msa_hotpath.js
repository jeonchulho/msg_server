import http from "k6/http";
import { check, fail, sleep } from "k6";
import { Trend } from "k6/metrics";

const CHAT_BASE_URL = __ENV.CHAT_BASE_URL || "http://localhost:8080";
const SESSION_BASE_URL = __ENV.SESSION_BASE_URL || "http://localhost:8090";
const ORGHUB_BASE_URL = __ENV.ORGHUB_BASE_URL || "http://localhost:8091";
const TENANTHUB_BASE_URL = __ENV.TENANTHUB_BASE_URL || "http://localhost:8092";

const TENANT_ID = __ENV.TENANT_ID || "default";
const EMAIL = __ENV.SMOKE_EMAIL || "admin@example.com";
const PASSWORD = __ENV.SMOKE_PASSWORD || "pass1234";

const K6_VUS = Number(__ENV.K6_VUS || "200");
const K6_DURATION = __ENV.K6_DURATION || "10m";
const K6_SLEEP_MS = Number(__ENV.K6_SLEEP_MS || "200");

const tOrghubLogin = new Trend("msa_orghub_login_ms");
const tChatCreateMessage = new Trend("msa_chat_create_message_ms");
const tSessionUpdateStatus = new Trend("msa_session_update_status_ms");
const tTenanthubList = new Trend("msa_tenanthub_list_ms");

export const options = {
  vus: K6_VUS,
  duration: K6_DURATION,
  thresholds: {
    http_req_failed: ["rate<0.02"],
    http_req_duration: ["p(95)<700", "p(99)<1500"],
    msa_chat_create_message_ms: ["p(95)<500", "p(99)<1200"],
    msa_session_update_status_ms: ["p(95)<500"],
    msa_tenanthub_list_ms: ["p(95)<600"],
  },
};

function authHeaders(token) {
  return {
    "Content-Type": "application/json",
    Authorization: `Bearer ${token}`,
  };
}

function postJSON(baseURL, path, payload, token) {
  const params = { headers: token ? authHeaders(token) : { "Content-Type": "application/json" } };
  return http.post(`${baseURL}${path}`, JSON.stringify(payload), params);
}

function patchJSON(baseURL, path, payload, token) {
  return http.patch(`${baseURL}${path}`, JSON.stringify(payload), { headers: authHeaders(token) });
}

function getJSON(baseURL, path, token) {
  return http.get(`${baseURL}${path}`, { headers: { Authorization: `Bearer ${token}` } });
}

export function setup() {
  const loginRes = postJSON(ORGHUB_BASE_URL, "/api/v1/auth/login", {
    tenant_id: TENANT_ID,
    email: EMAIL,
    password: PASSWORD,
  });

  tOrghubLogin.add(loginRes.timings.duration);

  const loginOK = check(loginRes, {
    "orghub login status 200": (r) => r.status === 200,
    "orghub login has token": (r) => {
      const body = r.json();
      return body && body.access_token;
    },
  });
  if (!loginOK) {
    fail(`orghub login failed: status=${loginRes.status} body=${loginRes.body}`);
  }

  const accessToken = loginRes.json("access_token");

  const roomRes = postJSON(
    CHAT_BASE_URL,
    "/api/v1/rooms",
    {
      name: "k6-msa-room",
      room_type: "group",
      member_ids: [],
    },
    accessToken
  );

  const roomOK = check(roomRes, {
    "chat create room status 201": (r) => r.status === 201,
    "chat create room has id": (r) => {
      const body = r.json();
      return body && body.id;
    },
  });
  if (!roomOK) {
    fail(`chat room create failed: status=${roomRes.status} body=${roomRes.body}`);
  }

  return {
    token: accessToken,
    roomId: String(roomRes.json("id")),
  };
}

export default function (data) {
  const messageRes = postJSON(
    CHAT_BASE_URL,
    `/api/v1/rooms/${encodeURIComponent(data.roomId)}/messages`,
    {
      body: `msa-k6-message-${__VU}-${__ITER}`,
      file_ids: [],
      emojis: [],
    },
    data.token
  );
  tChatCreateMessage.add(messageRes.timings.duration);

  const statusCycle = ["online", "busy", "away"];
  const selectedStatus = statusCycle[(__ITER + __VU) % statusCycle.length];
  const sessionRes = patchJSON(
    SESSION_BASE_URL,
    "/api/v1/session/status",
    {
      status: selectedStatus,
      status_note: `k6-${selectedStatus}-${__VU}-${__ITER}`,
    },
    data.token
  );
  tSessionUpdateStatus.add(sessionRes.timings.duration);

  const tenantRes = getJSON(TENANTHUB_BASE_URL, "/api/v1/tenants", data.token);
  tTenanthubList.add(tenantRes.timings.duration);

  check(messageRes, {
    "chat message status 201": (r) => r.status === 201,
  });
  check(sessionRes, {
    "session status update 200": (r) => r.status === 200,
  });
  check(tenantRes, {
    "tenanthub list status 200": (r) => r.status === 200,
  });

  if (K6_SLEEP_MS > 0) {
    sleep(K6_SLEEP_MS / 1000);
  }
}
