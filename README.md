# denotesrv

9P server for [denote](https://protesilaos.com/emacs/denote) notes.

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

## Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `DENOTE_DIR` | `~/Documents/notes` | Notes directory |
| `NAMESPACE` | `/tmp/ns.$USER.:0` | 9P namespace |

## 9P Filesystem

```
denote/
  ctl         (write) filter <query>, cd <path>
  event       (read)  Event stream
  index       (read)  Note metadata (respects filter)
  new         (write) Create note: "'title' tag1,tag2"
  n/
    <id>/
      backlinks (read)  Notes linking to this note
      ctl       (write) 'd' to delete
      keywords  (rw)    Tags
      path      (rw)    Filesystem path
      title     (rw)    Note title
```

## See Also

- [acme-denote](https://github.com/lneely/acme-denote) — Acme frontend
