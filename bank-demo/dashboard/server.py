import http.server
import socketserver
import json
import subprocess
import os

PORT = 3333

class Handler(http.server.SimpleHTTPRequestHandler):
    def do_GET(self):
        if self.path == '/api/state':
            self.send_response(200)
            self.send_header('Content-type', 'application/json')
            self.send_header('Access-Control-Allow-Origin', '*')
            self.end_headers()

            preview_groups = []

            # Source 1: PreviewGroup CRDs (controller-managed)
            try:
                pg_out = subprocess.check_output(['kubectl', 'get', 'previewgroup', '-o', 'json'], stderr=subprocess.DEVNULL)
                pg_data = json.loads(pg_out)
                seen_names = set()
                for item in pg_data.get('items', []):
                    spec = item.get('spec', {})
                    name = item.get('metadata', {}).get('name', '')
                    seen_names.add(name)
                    services = []
                    for s in spec.get('services', []):
                        svc_mode = s.get('mode', 'image')
                        endpoint = s.get('endpoint', '')
                        if not endpoint and svc_mode == 'local':
                            endpoint = '100.86.105.10:9090'
                        services.append({
                            "name": s.get('name'),
                            "namespace": s.get('namespace', 'demo-bank'),
                            "mode": svc_mode,
                            "phase": item.get('status', {}).get('phase', 'Running'),
                            "endpoint": endpoint
                        })
                    routing = spec.get('routing', {})
                    preview_groups.append({
                        "name": name,
                        "phase": item.get('status', {}).get('phase', 'Pending'),
                        "owner": spec.get('owner', item.get('metadata', {}).get('annotations', {}).get('diverge.dev/owner', 'unknown')),
                        "header": routing.get('headerValue', routing.get('headerKey', '')),
                        "services": services
                    })
            except Exception:
                seen_names = set()

            # Source 2: HTTPRoutes with diverge pattern (manual / ambient routing)
            try:
                hr_out = subprocess.check_output(
                    ['kubectl', 'get', 'httproute', '-n', 'demo-bank', '-o', 'json'],
                    stderr=subprocess.DEVNULL
                )
                hr_data = json.loads(hr_out)
                for item in hr_data.get('items', []):
                    name = item.get('metadata', {}).get('name', '')
                    if not name.endswith('-diverge'):
                        continue
                    svc_name = name.replace('-diverge', '')

                    # Check if already covered by a PreviewGroup
                    already_covered = False
                    for pg in preview_groups:
                        for s in pg.get('services', []):
                            if s.get('name') == svc_name:
                                already_covered = True
                                break
                    if already_covered:
                        continue

                    # Extract header value and backend from rules
                    header_val = ''
                    mode = 'image'
                    endpoint = ''
                    backend_svc = ''
                    for rule in item.get('spec', {}).get('rules', []):
                        for match in rule.get('matches', []):
                            for h in match.get('headers', []):
                                if h.get('name') == 'x-diverge-env':
                                    header_val = h.get('value', '')
                        for ref in rule.get('backendRefs', []):
                            ref_name = ref.get('name', '')
                            if ref_name != svc_name:
                                backend_svc = ref_name

                    # Check if backend points to a Tailscale IP (local mode)
                    if backend_svc:
                        try:
                            es_out = subprocess.check_output(
                                ['kubectl', 'get', 'endpointslice', '-n', 'demo-bank',
                                 '-l', f'kubernetes.io/service-name={backend_svc}',
                                 '-o', 'jsonpath={.items[0].endpoints[0].addresses[0]}'],
                                stderr=subprocess.DEVNULL
                            )
                            ep_ip = es_out.decode().strip()
                            if ep_ip.startswith('100.'):
                                mode = 'local'
                                port_out = subprocess.check_output(
                                    ['kubectl', 'get', 'endpointslice', '-n', 'demo-bank',
                                     '-l', f'kubernetes.io/service-name={backend_svc}',
                                     '-o', 'jsonpath={.items[0].ports[0].port}'],
                                    stderr=subprocess.DEVNULL
                                )
                                endpoint = f"{ep_ip}:{port_out.decode().strip()}"
                        except Exception:
                            pass

                    preview_groups.append({
                        "name": f"dev-{header_val}" if mode == 'local' else f"preview-{header_val}",
                        "phase": "Running",
                        "owner": "ab" if mode == 'local' else "team",
                        "header": header_val,
                        "services": [{
                            "name": svc_name,
                            "namespace": "demo-bank",
                            "mode": mode,
                            "phase": "Running",
                            "endpoint": endpoint
                        }]
                    })
            except Exception:
                pass

            # Baseline services from pods
            baseline_services = []
            try:
                pods_out = subprocess.check_output(
                    ['kubectl', 'get', 'pods', '-n', 'demo-bank',
                     '-l', 'app', '-o', 'json'],
                    stderr=subprocess.DEVNULL
                )
                pods_data = json.loads(pods_out)
                seen_apps = set()
                baseline_apps = {'gateway', 'payments-api', 'accounts-api', 'payments-module', 'web-app'}
                for pod in pods_data.get('items', []):
                    app = pod.get('metadata', {}).get('labels', {}).get('app', '')
                    if app in baseline_apps and app not in seen_apps:
                        seen_apps.add(app)
                        ready = all(
                            c.get('ready', False)
                            for c in pod.get('status', {}).get('containerStatuses', [])
                        )
                        baseline_services.append({
                            "name": app,
                            "namespace": "demo-bank",
                            "ready": ready
                        })
            except Exception:
                baseline_services = [
                    {"name": n, "namespace": "demo-bank", "ready": True}
                    for n in ["gateway", "payments-api", "accounts-api", "payments-module", "web-app"]
                ]

            state = {
                "cluster": "diverge-demo-arm",
                "previewGroups": preview_groups,
                "baselineServices": baseline_services,
                "tailscale": {
                    "clusterIP": "100.73.221.113",
                    "macIP": "100.86.105.10",
                    "connection": "direct"
                }
            }

            self.wfile.write(json.dumps(state).encode())
        else:
            if self.path == '/':
                self.path = '/index.html'
            super().do_GET()

with socketserver.TCPServer(("", PORT), Handler) as httpd:
    print(f"Serving on port {PORT}")
    httpd.serve_forever()
