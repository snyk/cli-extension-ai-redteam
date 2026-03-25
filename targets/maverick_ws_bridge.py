#!/usr/bin/env python3
"""
WebSocket bridge for maverick-ui chat via target_command interface.

Receives JSON on stdin:  {"chat_id": "...", "seq": N, "prompt": "..."}
Returns JSON on stdout:  {"response": "..."}

Connects to the maverick-ui Socket.IO endpoint, sends the prompt as a user
message, collects streaming response events, and returns the final text.

Environment variables:
  MAVERICK_URL       - Backend URL (default: http://localhost:8000)
  MAVERICK_TENANT_ID - Tenant UUID (default: demo tenant 9900b2b0-...)
  MAVERICK_TIMEOUT   - Response timeout in seconds (default: 120)

Assumptions (local dev only):
  - Connects directly to the backend at port 8000, bypassing the Node.js
    server (port 9090) which would require cookie-based auth (connect.sid).
  - The backend requires an Authorization header but does NOT validate the
    token when the server config has skipAuth: true (see
    packages/server/config/config.secret.json). We send a random UUID as the
    Bearer token — this will NOT work in production or any env where auth is
    enforced.
  - The demo tenant ID (9900b2b0-...) is hardcoded in the maverick-ui
    accounts-service.ts and only exists in the local dev seed data. Override
    via MAVERICK_TENANT_ID for other environments.
  - skipAuthZ: true in the secret config means authorization checks are also
    bypassed, so any user/tenant combination is accepted.
"""

import json
import os
import sys
import threading
import uuid

import socketio

# Demo tenant from maverick-ui accounts-service.ts
DEFAULT_TENANT_ID = "9900b2b0-cea4-472a-a33b-2478c74552d5"


def main():
    req = json.loads(sys.stdin.read())
    chat_id = req.get("chat_id", str(uuid.uuid4()))
    prompt = req["prompt"]

    # Direct backend connection — bypasses the Node.js auth layer at :9090.
    base_url = os.environ.get("MAVERICK_URL", "http://localhost:8000")
    tenant_id = os.environ.get("MAVERICK_TENANT_ID", DEFAULT_TENANT_ID)
    timeout = int(os.environ.get("MAVERICK_TIMEOUT", "120"))

    socket_path = f"/hidden/tenants/{tenant_id}/evo/sockets/socket.io/"

    # Use chat_id as the Socket.IO session_id for session affinity.
    session_id = chat_id

    sio = socketio.Client(
        logger=False,
        engineio_logger=False,
    )

    response_chunks: list[str] = []
    done_event = threading.Event()
    error_msg: list[str] = []

    @sio.on("message")
    def on_message(data):
        if isinstance(data, str):
            try:
                data = json.loads(data)
            except json.JSONDecodeError:
                return

        if not isinstance(data, dict):
            return

        msg_type = data.get("type", "")
        content = data.get("content", "")

        # Streaming text delta — content field holds the chunk.
        if msg_type == "response.output_text.delta":
            if content:
                response_chunks.append(content)

        # Full text response (non-streaming).
        elif msg_type == "text" and data.get("role") == "agent":
            if content:
                response_chunks.append(content)

        # Response complete.
        elif msg_type == "response.completed":
            done_event.set()

    @sio.on("connect_error")
    def on_connect_error(data):
        error_msg.append(f"connection error: {data}")
        done_event.set()

    @sio.on("disconnect")
    def on_disconnect():
        if not done_event.is_set():
            error_msg.append("disconnected before response completed")
            done_event.set()

    try:
        sio.connect(
            base_url,
            socketio_path=socket_path,
            headers={
                # Backend requires a Bearer token but does not validate it when
                # skipAuth is true. A random UUID satisfies the header check.
                # In production, this must be a real token from the auth flow.
                "Authorization": f"Bearer {uuid.uuid4()}",
            },
            transports=["websocket"],
            wait_timeout=30,
        )
    except Exception as e:
        print(f"failed to connect to {base_url}: {e}", file=sys.stderr)
        sys.exit(1)

    # Send the user message.
    message = {
        "id": str(uuid.uuid4()),
        "role": "user",
        "type": "text",
        "content": prompt,
        "session_id": session_id,
    }
    sio.emit("message", message)

    # Wait for the response to complete.
    done_event.wait(timeout=timeout)

    sio.disconnect()

    if error_msg:
        print(error_msg[0], file=sys.stderr)
        sys.exit(1)

    if not response_chunks:
        print("no response received within timeout", file=sys.stderr)
        sys.exit(1)

    response_text = "".join(response_chunks)
    print(json.dumps({"response": response_text}))


if __name__ == "__main__":
    main()
