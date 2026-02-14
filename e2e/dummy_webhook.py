import http.server
import socketserver

PORT = 18082

class H(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        # accept any payload
        self.send_response(200)
        self.send_header('content-type', 'application/json')
        self.end_headers()
        self.wfile.write(b'{"ok":true}')

    def log_message(self, format, *args):
        return

if __name__ == '__main__':
    with socketserver.TCPServer(('127.0.0.1', PORT), H) as httpd:
        httpd.serve_forever()
