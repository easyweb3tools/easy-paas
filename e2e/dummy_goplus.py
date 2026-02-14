import http.server
import socketserver
import urllib.parse
import json

PORT = 18087

class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path.startswith('/api/v1/token_security/'):
            chain_id = self.path.split('/api/v1/token_security/', 1)[1].split('?', 1)[0]
            qs = urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query)
            addrs = qs.get('contract_addresses', [''])[0]
            self.send_response(200)
            self.send_header('content-type', 'application/json')
            self.end_headers()
            body = {
                'code': 1,
                'message': 'OK',
                'result': {
                    addrs: {
                        'chain_id': chain_id,
                        'contract_address': addrs,
                        'is_honeypot': '0'
                    }
                }
            }
            self.wfile.write(json.dumps(body).encode('utf-8'))
            return
        self.send_response(404)
        self.end_headers()

    def log_message(self, format, *args):
        return

if __name__ == '__main__':
    with socketserver.TCPServer(('127.0.0.1', PORT), H) as httpd:
        httpd.serve_forever()
