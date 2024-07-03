# gluetun-qbit-port
This is a Docker sidecar container for Gluetun and qBittorrent. It ensures that qBit remains configured to listen on the port forwarded from Gluetun through the API for each of the services.

## Setup
You can use the provided `compose.yml` as a starting point for your existing Docker Compose stack if that's your preferred method of running containers. Since the networking for this container can get a bit complicated, I _highly_ recommend using Compose instead of the Docker CLI when running this container.

### Networking
The important thing to make this work is to make sure that this container can access the APIs for both Gluetun and qBittorrent. The default assumption is that you'll do this by putting this container behind Gluetun alongside qBittorrent. In theory, you could expose the APIs for Gluetun and qBit to a subnet and access them both from outside of Gluetun if your ports are exposed correctly, but that can get a bit tricky. If you're unsure, it's easiest to just attach this container to Gluetun alongside qBittorrent with the `network_mode: "service:gluetun"` line in the `compose.yml`.

### Variables
Use the provided `example.env` as a starting point for your connection parameters and such. This is where you'll set your usernames, passwords, and URLs if necessary to connect to the two services. Copy `example.env` into `.env` and set those values in your new `.env` file. You can leave the `compose.yml` alone and just set the values in `.env` or you can set them directly in the `compose.yml`. I personally like to see what environment variables I'm putting in each container without exposing secrets in my orchestration file, but you do what you want.