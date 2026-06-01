`root@eco:~# cat /Container/media/gluetun_config/auth/config.toml`

```toml
[[roles]]
name = "portforward"
# Define a list of routes with the syntax "Http-Method /path"
routes = ["GET /v1/portforward", "GET /v1/vpn/status"]
# Define an authentication method with its parameters
auth = "apikey"
# docker run --rm qmcgaw/gluetun genkey
apikey = "6HKNBbjAoCQdTxKn55M1yw"

[[roles]]
name = "gluetunrestart"
routes = ["PUT /v1/vpn/status", "GET /v1/publicip/ip"]
auth = "basic"
username = "myusername"
password = "mypassword"
```

https://github.com/qdm12/gluetun-wiki/blob/main/setup/advanced/control-server.md