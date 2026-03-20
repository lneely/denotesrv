# denotesrv

9P server for [denote](https://protesilaos.com/emacs/denote) notes.

## Dependencies

- [plan9port](https://github.com/lneely/plan9port) (wayland branch required for `9pfuse` truncate fix)

## Installation

```sh
mk install
```

## Usage

```sh
denotesrv start       # background
denotesrv fgstart     # foreground
denotesrv status
denotesrv stop
```

On startup, the server automatically mounts at `~/mnt/denote` via 9pfuse.
Use `-mount /path` to override the mount location.

## Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `DENOTE_DIR` | `~/Documents/notes` | Notes directory |
| `NAMESPACE` | `/tmp/ns.$USER.:0` | 9P namespace |

## 9P Filesystem

```
denote/
  ctl         (write) filter <query>, cd <path>, refresh
  event       (read)  Event stream
  index       (read)  Note metadata (respects filter)
  new         (write) Create note: "'title' tag1,tag2"
  n/
    <id>/
      backlinks (read)  Notes linking to this note
      body      (rw)    File content
      ctl       (write) 'd' to delete
      keywords  (rw)    Tags
      path      (rw)    Filesystem path
      signature (rw)    Note signature
      title     (rw)    Note title
```

## See Also

- [acme-denote](https://github.com/lneely/acme-denote) — Acme frontend
