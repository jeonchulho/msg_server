#!/usr/bin/env bash

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
TENANT_ID="${TENANT_ID:-default}"
SMOKE_EMAIL="${SMOKE_EMAIL:-admin@example.com}"
SMOKE_PASSWORD="${SMOKE_PASSWORD:-${SMOKE_ADMIN_PASSWORD:-pass1234}}"

echo "[info] ws-only smoke start"
echo "[info] base_url=${BASE_URL} tenant_id=${TENANT_ID} email=${SMOKE_EMAIL}"
echo "[info] prerequisite: chat server must run with CHAT_USE_MQ=false"

if ! curl -fsS "${BASE_URL}/health" >/dev/null; then
  echo "[error] chat server is not reachable: ${BASE_URL}"
  echo "[hint] run: CHAT_USE_MQ=false make run-chat"
  exit 1
fi

tmp_file="$(mktemp /tmp/chat_ws_only_smoke_XXXXXX.go)"
trap 'rm -f "${tmp_file}"' EXIT

cat >"${tmp_file}" <<'EOF'
package main

import (
  "bytes"
  "encoding/json"
  "fmt"
  "io"
  "net/http"
  "net/url"
  "os"
  "strings"
  "time"

  "github.com/gorilla/websocket"
)

type loginResponse struct {
  AccessToken string `json:"access_token"`
}

type idResponse struct {
  ID any `json:"id"`
}

type messagePayload struct {
  Body string `json:"body"`
}

type wsEnvelope struct {
  Type    string         `json:"type"`
  Payload messagePayload `json:"payload"`
}

func postJSON(baseURL, path string, token string, body any, out any) error {
  raw, err := json.Marshal(body)
  if err != nil {
    return err
  }
  req, err := http.NewRequest(http.MethodPost, strings.TrimRight(baseURL, "/")+path, bytes.NewReader(raw))
  if err != nil {
    return err
  }
  req.Header.Set("Content-Type", "application/json")
  if token != "" {
    req.Header.Set("Authorization", "Bearer "+token)
  }

  client := &http.Client{Timeout: 10 * time.Second}
  resp, err := client.Do(req)
  if err != nil {
    return err
  }
  defer resp.Body.Close()
  if resp.StatusCode < 200 || resp.StatusCode >= 300 {
    b, _ := io.ReadAll(resp.Body)
    return fmt.Errorf("%s %s failed: status=%d body=%s", http.MethodPost, path, resp.StatusCode, string(b))
  }
  if out == nil {
    return nil
  }
  return json.NewDecoder(resp.Body).Decode(out)
}

func wsURL(baseURL, roomID, token string) (string, error) {
  u, err := url.Parse(baseURL)
  if err != nil {
    return "", err
  }
  switch u.Scheme {
  case "http":
    u.Scheme = "ws"
  case "https":
    u.Scheme = "wss"
  default:
    return "", fmt.Errorf("unsupported scheme: %s", u.Scheme)
  }
  u.Path = "/ws"
  q := u.Query()
  q.Set("room_id", roomID)
  q.Set("access_token", token)
  u.RawQuery = q.Encode()
  return u.String(), nil
}

func main() {
  baseURL := os.Getenv("BASE_URL")
  tenantID := os.Getenv("TENANT_ID")
  email := os.Getenv("SMOKE_EMAIL")
  password := os.Getenv("SMOKE_PASSWORD")
  if baseURL == "" || tenantID == "" || email == "" || password == "" {
    fmt.Println("WS_ONLY_SMOKE_FAILED: required env BASE_URL/TENANT_ID/SMOKE_EMAIL/SMOKE_PASSWORD")
    os.Exit(1)
  }

  bodyText := "mq-off-rest-to-ws-smoke-" + time.Now().UTC().Format("20060102150405")

  var login loginResponse
  if err := postJSON(baseURL, "/api/v1/auth/login", "", map[string]any{
    "tenant_id": tenantID,
    "email":     email,
    "password":  password,
  }, &login); err != nil {
    fmt.Printf("WS_ONLY_SMOKE_FAILED: login: %v\n", err)
    os.Exit(1)
  }
  if strings.TrimSpace(login.AccessToken) == "" {
    fmt.Println("WS_ONLY_SMOKE_FAILED: login returned empty access_token")
    os.Exit(1)
  }

  var room idResponse
  if err := postJSON(baseURL, "/api/v1/rooms", login.AccessToken, map[string]any{
    "name":       "mq-off-smoke-room",
    "room_type":  "group",
    "member_ids": []string{},
  }, &room); err != nil {
    fmt.Printf("WS_ONLY_SMOKE_FAILED: create room: %v\n", err)
    os.Exit(1)
  }

  roomID := strings.TrimSpace(fmt.Sprint(room.ID))
  if roomID == "" || roomID == "<nil>" {
    fmt.Println("WS_ONLY_SMOKE_FAILED: invalid room id")
    os.Exit(1)
  }

  wsAddr, err := wsURL(baseURL, roomID, login.AccessToken)
  if err != nil {
    fmt.Printf("WS_ONLY_SMOKE_FAILED: ws url: %v\n", err)
    os.Exit(1)
  }

  conn, _, err := websocket.DefaultDialer.Dial(wsAddr, nil)
  if err != nil {
    fmt.Printf("WS_ONLY_SMOKE_FAILED: ws dial: %v\n", err)
    os.Exit(1)
  }
  defer conn.Close()

  readDone := make(chan error, 1)
  go func() {
    deadline := time.Now().Add(8 * time.Second)
    for {
      _ = conn.SetReadDeadline(deadline)
      _, msg, err := conn.ReadMessage()
      if err != nil {
        readDone <- fmt.Errorf("ws read: %w", err)
        return
      }
      var env wsEnvelope
      if err := json.Unmarshal(msg, &env); err != nil {
        continue
      }
      if env.Type == "message" && env.Payload.Body == bodyText {
        readDone <- nil
        return
      }
    }
  }()

  if err := postJSON(baseURL, "/api/v1/rooms/"+roomID+"/messages", login.AccessToken, map[string]any{
    "body":    bodyText,
    "file_ids": []string{},
    "emojis":  []string{},
  }, nil); err != nil {
    fmt.Printf("WS_ONLY_SMOKE_FAILED: create message: %v\n", err)
    os.Exit(1)
  }

  if err := <-readDone; err != nil {
    fmt.Printf("WS_ONLY_SMOKE_FAILED: %v\n", err)
    os.Exit(1)
  }

  fmt.Println("WS_ONLY_SMOKE_OK")
}
EOF

BASE_URL="${BASE_URL}" \
TENANT_ID="${TENANT_ID}" \
SMOKE_EMAIL="${SMOKE_EMAIL}" \
SMOKE_PASSWORD="${SMOKE_PASSWORD}" \
go run "${tmp_file}"

echo "[ok] ws-only smoke passed"
