import http.server
import socketserver
import urllib.parse

PORT = 18083

class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        # Minimal subset
        if self.path.startswith('/latest/dex/search'):
            q = urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query).get('q', [''])[0]
            self.send_response(200)
            self.send_header('content-type', 'application/json')
            self.end_headers()
            self.wfile.write((
                '{"schemaVersion":"1","q":%s,"pairs":[]}'
                % (('"%s"' % q.replace('"','')))
            ).encode('utf-8'))
            return
        if self.path.startswith('/latest/dex/pairs/'):
            self.send_response(200)
            self.send_header('content-type', 'application/json')
            self.end_headers()
            self.wfile.write(b'{"schemaVersion":"1","pairs":[]}')
            return
        if self.path.startswith('/latest/dex/tokens/'):
            self.send_response(200)
            self.send_header('content-type', 'application/json')
            self.end_headers()
            self.wfile.write(b'{"schemaVersion":"1","pairs":[]}')
            return
        self.send_response(404)
        self.end_headers()

    def log_message(self, format, *args):
        return

if __name__ == '__main__':
    with socketserver.TCPServer(('127.0.0.1', PORT), H) as httpd:
        httpd.serve_forever()
