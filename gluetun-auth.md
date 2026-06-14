`root@eco:~# cat /Container/media/gluetun_config/auth/config.toml`

```toml
[[roles]]
name = "portforward"
# Define a list of routes with the syntax "Http-Method /path"
routes = ["GET /v1/portforward", "GET /v1/vpn/status"]
# Define an authentication method with its parameters
auth = "apikey"
# docker run --rm qmcgaw/gluetun genkey
apikey = "qwertyuiopasdfghjklzxcvbnm"
```

https://github.com/qdm12/gluetun-wiki/blob/main/setup/advanced/control-server.md
