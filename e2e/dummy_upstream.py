import http.server
import socketserver

PORT = 18081

class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == '/health':
            self.send_response(200)
            self.send_header('content-type', 'application/json')
            self.end_headers()
            self.wfile.write(b'{"status":"ok"}')
            return
        if self.path == '/docs':
            self.send_response(200)
            self.send_header('content-type', 'text/markdown; charset=utf-8')
            self.end_headers()
            self.wfile.write(b'# Dummy Meme Service\n\n- GET /health\n')
            return
        self.send_response(404)
        self.end_headers()

    def log_message(self, format, *args):
        return

if __name__ == '__main__':
    with socketserver.TCPServer(('127.0.0.1', PORT), H) as httpd:
        httpd.serve_forever()
