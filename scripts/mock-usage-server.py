#!/usr/bin/env python3

import argparse
import json
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def build_payload(account_id: str) -> dict:
    now = int(time.time())

    usage_by_account = {
        "acct_demo_plus": {
            "plan_type": "plus",
            "allowed": True,
            "limit_reached": False,
            "primary_window": {
                "limit_window_seconds": 18000,
                "used_percent": 34.0,
                "reset_at": now + 105 * 60,
            },
            "secondary_window": {
                "limit_window_seconds": 604800,
                "used_percent": 57.0,
                "reset_at": now + 4 * 24 * 60 * 60,
            },
        },
        "acct_demo_free_active": {
            "plan_type": "free",
            "allowed": True,
            "limit_reached": False,
            "primary_window": None,
            "secondary_window": {
                "limit_window_seconds": 604800,
                "used_percent": 64.0,
                "reset_at": now + 2 * 24 * 60 * 60,
            },
        },
        "acct_demo_free_alpha": {
            "plan_type": "free",
            "allowed": True,
            "limit_reached": False,
            "primary_window": None,
            "secondary_window": {
                "limit_window_seconds": 604800,
                "used_percent": 24.0,
                "reset_at": now + 6 * 24 * 60 * 60,
            },
        },
        "acct_demo_free_beta": {
            "plan_type": "free",
            "allowed": True,
            "limit_reached": False,
            "primary_window": None,
            "secondary_window": {
                "limit_window_seconds": 604800,
                "used_percent": 81.0,
                "reset_at": now + 24 * 60 * 60,
            },
        },
        "acct_demo_free_gamma": {
            "plan_type": "free",
            "allowed": False,
            "limit_reached": True,
            "primary_window": None,
            "secondary_window": {
                "limit_window_seconds": 604800,
                "used_percent": 100.0,
                "reset_at": now + 16 * 60 * 60,
            },
        },
        "acct_demo_free_delta": {
            "plan_type": "free",
            "allowed": False,
            "limit_reached": True,
            "primary_window": None,
            "secondary_window": {
                "limit_window_seconds": 604800,
                "used_percent": 100.0,
                "reset_at": now + 10 * 60 * 60,
            },
        },
        "acct_demo_free_epsilon": {
            "plan_type": "free",
            "allowed": True,
            "limit_reached": False,
            "primary_window": None,
            "secondary_window": {
                "limit_window_seconds": 604800,
                "used_percent": 46.0,
                "reset_at": now + 4 * 24 * 60 * 60,
            },
        },
        "acct_demo_free_zeta": {
            "plan_type": "free",
            "allowed": False,
            "limit_reached": True,
            "primary_window": None,
            "secondary_window": {
                "limit_window_seconds": 604800,
                "used_percent": 100.0,
                "reset_at": now + 7 * 60 * 60,
            },
        },
        "acct_demo_free_eta": {
            "plan_type": "free",
            "allowed": True,
            "limit_reached": False,
            "primary_window": None,
            "secondary_window": {
                "limit_window_seconds": 604800,
                "used_percent": 12.0,
                "reset_at": now + 3 * 24 * 60 * 60,
            },
        },
        "acct_demo_free_theta": {
            "plan_type": "free",
            "allowed": False,
            "limit_reached": True,
            "primary_window": None,
            "secondary_window": {
                "limit_window_seconds": 604800,
                "used_percent": 100.0,
                "reset_at": now + 5 * 60 * 60,
            },
        },
    }

    usage = usage_by_account.get(
        account_id,
        {
            "plan_type": "free",
            "allowed": True,
            "limit_reached": False,
            "primary_window": None,
            "secondary_window": {
                "limit_window_seconds": 604800,
                "used_percent": 50.0,
                "reset_at": now + 2 * 24 * 60 * 60,
            },
        },
    )

    return {
        "plan_type": usage["plan_type"],
        "rate_limit": {
            "allowed": usage["allowed"],
            "limit_reached": usage["limit_reached"],
            "primary_window": usage["primary_window"],
            "secondary_window": usage["secondary_window"],
        },
    }


class Handler(BaseHTTPRequestHandler):
    delay_ms = 200

    def do_GET(self):
        if self.path != "/backend-api/wham/usage":
            self.send_response(404)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"error":"not found"}')
            return

        time.sleep(max(self.delay_ms, 0) / 1000.0)
        account_id = self.headers.get("ChatGPT-Account-Id", "").strip()
        payload = build_payload(account_id)
        body = json.dumps(payload).encode("utf-8")

        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format: str, *args):
        return


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=18080)
    parser.add_argument("--delay-ms", type=int, default=200)
    args = parser.parse_args()

    Handler.delay_ms = args.delay_ms
    server = ThreadingHTTPServer((args.host, args.port), Handler)
    server.serve_forever()


if __name__ == "__main__":
    main()
