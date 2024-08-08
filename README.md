Solution to use static macvlan IPs in Docker swarm

I have not tested this in a multi node swarp yet, so don't expect it to work perfectly

### Usage
You need to have the desired IP address set as the alias in the network you are using,

for example, in docker compose:

```YAML
networks:
  external-net:
    external: true
services:
  plex:
    image: ...
    ...
    networks:
      default:    # need this to use default alias in the default network
      external-net:
        aliases:
          - 192.168.1.8
  swarm-macvlan:
    image: ethanscully/macvlan
    deploy:
      mode: global
      placement:
        constraints:
          - node.role == manager
    volumes: 
      - /var/run/docker.sock:/var/run/docker.sock

```
To set up the network for example:
```SHELL
docker network create --config-only --subnet 192.168.1.0/24 --gateway 192.168.1.1 -o parent=eno1 external-config
docker network create -d macvlan --scope=swarm --config-from external-config external-net
```
