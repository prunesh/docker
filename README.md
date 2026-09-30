# prunesh/docker

Token-reduction plugin for [prunesh](https://github.com/prunesh/prunesh) that filters `docker` output.

`docker inspect` can produce 200+ lines of JSON containing `GraphDriver`, `HostConfig`, `NetworkSettings`, mount details, and internal paths that are rarely useful in an AI coding session. `docker logs` without a tail limit can dump thousands of lines. This plugin strips the noise and caps what reaches the context window.

## What it filters

| Subcommand | Action |
|---|---|
| `inspect` | Drops `GraphDriver`, `HostConfig`, `Mounts`, `NetworkSettings`, `LogPath`, `ResolvConfPath`, `HostnamePath`, `MountLabel`, `ProcessLabel`, `AppArmorProfile`, `ExecIDs`. Keeps `State`, `Config`, `Image`, `Name`, `Id`. |
| `logs` | Rewrite adds `--tail=100` when no `--tail` or `-f` flag is present. |
| `pull` | Strips per-layer progress lines (`Pulling fs layer`, `Waiting`, `Downloading`, `Verifying Checksum`, `Pull complete`, …). Keeps `Status:` and the final image reference. |
| `images` | Caps table at 30 rows + `... +N more images`. |
| Everything else | Full passthrough (`ps`, `run`, `exec`, `build`, `push`, …). |

### Example — inspect

**Before** (excerpt):
```json
"GraphDriver": {
    "Data": {
        "LowerDir": "/var/lib/docker/overlay2/abc/diff",
        "MergedDir": "/var/lib/docker/overlay2/abc/merged",
        "UpperDir": "/var/lib/docker/overlay2/abc/upper",
        "WorkDir": "/var/lib/docker/overlay2/abc/work"
    },
    "Name": "overlay2"
},
"HostConfig": {
    "Binds": null,
    "ContainerIDFile": "",
    "LogConfig": { "Type": "json-file", "Config": {} },
    "NetworkMode": "default",
    ...dozens more fields...
},
"NetworkSettings": { ...30+ fields... }
```

**After**: those blocks are gone. `State`, `Config`, and `Name` stay — they are the first thing to check when debugging a container.

### Example — pull

**Before**:
```
latest: Pulling from library/nginx
Pulling fs layer
Waiting
Downloading   1.23MB/50.00MB
Verifying Checksum
Download complete
Pull complete
...
Status: Downloaded newer image for nginx:latest
docker.io/library/nginx:latest
```

**After**:
```
Status: Downloaded newer image for nginx:latest
docker.io/library/nginx:latest
```

## Install

Requires [prunesh core](https://github.com/prunesh/prunesh) >= 0.16.0.

```bash
prunesh plugin install github.com/prunesh/docker@v0.3.0
```

To replace an existing `docker` plugin:

```bash
prunesh plugin install github.com/prunesh/docker@v0.3.0 --replace
```

## Uninstall

```bash
prunesh plugin uninstall prunesh/docker
```

## How it works

- **Rewrite**: adds `--tail=100` to `docker logs` when no tail limit or follow flag is present.
- **FilterOutput**: block-skip filter for `inspect` (same JSON key-based approach as `kubectl get -o json`); prefix-match filter for `pull` progress lines; row cap for `images`.

## License

MIT
