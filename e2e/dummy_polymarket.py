import http.server
import socketserver
import json
import urllib.parse

PORT = 18081


def _json(w, status, obj):
    w.send_response(status)
    w.send_header("content-type", "application/json")
    w.end_headers()
    w.wfile.write(json.dumps(obj).encode("utf-8"))


class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/healthz":
            _json(self, 200, {"status": "ok"})
            return
        if self.path == "/docs":
            self.send_response(200)
            self.send_header("content-type", "text/markdown; charset=utf-8")
            self.end_headers()
            self.wfile.write(
                b"# Dummy Polymarket Service\n\n- GET /healthz\n- GET /api/v2/opportunities\n"
            )
            return

        # Minimal subset to support cli integration smoke.
        if self.path.startswith("/api/v2/opportunities"):
            qs = urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query)
            limit = int(qs.get("limit", ["50"])[0])
            _json(
                self,
                200,
                {
                    "items": [
                        {
                            "id": "opp_1",
                            "title": "Dummy opportunity",
                            "score": 0.1,
                            "status": "active",
                        }
                    ][:limit],
                    "next_offset": 0,
                },
            )
            return

        if self.path.startswith("/api/v2/executions"):
            _json(self, 200, {"items": [], "next_offset": 0})
            return

        if self.path.startswith("/api/catalog/events"):
            _json(self, 200, {"items": [], "next_offset": 0})
            return

        if self.path.startswith("/api/catalog/markets"):
            _json(self, 200, {"items": [], "next_offset": 0})
            return

        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        # Minimal subset to support cli integration smoke.
        if self.path.startswith("/api/catalog/sync"):
            _json(self, 200, {"ok": True})
            return
        if self.path.endswith("/dismiss") or self.path.endswith("/execute"):
            _json(self, 200, {"ok": True})
            return
        if self.path.endswith("/preflight"):
            _json(self, 200, {"ok": True})
            return
        if self.path.endswith("/mark-executing") or self.path.endswith("/mark-executed") or self.path.endswith("/cancel"):
            _json(self, 200, {"ok": True})
            return
        if self.path.endswith("/fill") or self.path.endswith("/settle"):
            _json(self, 200, {"ok": True})
            return

        self.send_response(404)
        self.end_headers()

    def log_message(self, format, *args):
        return


if __name__ == "__main__":
    with socketserver.TCPServer(("127.0.0.1", PORT), H) as httpd:
        httpd.serve_forever()
