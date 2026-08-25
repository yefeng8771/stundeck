#!/bin/bash
set -u

port_file=/var/apps/stundeck-fpk/etc/stundeck.port

bad_request() {
    printf 'Status: 400 Bad Request\r\n'
    printf 'Content-Type: text/plain; charset=utf-8\r\n'
    printf 'Cache-Control: no-store\r\n\r\n'
    printf '%s\n' "$1"
    exit 0
}

[ -r "$port_file" ] || bad_request "StunDeck 端口记录不存在。"
IFS= read -r port < "$port_file" || bad_request "StunDeck 端口记录读取失败。"
case "$port" in
    ''|*[!0-9]*) bad_request "StunDeck 端口记录无效。" ;;
esac
[ "$port" -ge 1024 ] 2>/dev/null && [ "$port" -le 65535 ] 2>/dev/null \
    || bad_request "StunDeck 端口记录超出范围。"

# fnOS invokes third-party CGI through an internal localhost gateway, so
# HTTP_HOST/SERVER_NAME can describe the gateway instead of the NAS address that
# the browser used. Resolve the destination in the browser to retain that public
# hostname (including IPv6 and reverse-proxy hostnames), while the server only
# interpolates the already validated numeric port.
printf 'Status: 200 OK\r\n'
printf 'Content-Type: text/html; charset=utf-8\r\n'
printf 'Cache-Control: no-store\r\n'
printf "Content-Security-Policy: default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'\r\n\r\n"
cat <<EOF
<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>正在打开 StunDeck</title>
  <style>body{font:16px system-ui,sans-serif;margin:3rem;color:#1f2937}</style>
</head>
<body>
  <p>正在打开 StunDeck……</p>
  <script>
    const target = new URL('/', window.location.href);
    target.protocol = 'http:';
    target.port = '$port';
    window.location.replace(target.href);
  </script>
</body>
</html>
EOF
