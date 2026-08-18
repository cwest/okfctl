# Remote sources: `registry` and `connect`

okfctl treats remote bundles the way git treats remotes: a named source is a
plain git URL. This is `git remote` for OKF bundles—**not** a hosted service,
account system, or schema registry. Named sources live in the one okfctl config
store (keyed `registry.<name>`); there's no second config file.

## Register a source

```sh
okfctl registry add knowledge https://github.com/cwest/knowledge-base.git
okfctl registry list                 # name -> url, sorted by name
okfctl registry show knowledge       # print one source's URL
okfctl registry remove knowledge     # (alias: rm) unregister
```

## Materialize a source locally

```sh
okfctl connect knowledge             # a registered name resolves to its URL
okfctl connect https://github.com/cwest/knowledge-base.git   # or an ad-hoc git URL
okfctl connect knowledge ./kb        # explicit destination directory
```

`connect` behavior:

- A fresh destination is `git clone`d.
- An existing checkout of the same source is fast-forwarded (`git pull
  --ff-only`, never a history-rewriting merge).
- A non-empty directory that isn't that checkout is left untouched.

okfctl shells out to `git` (no new dependency) and does no authentication of its
own—reaching a private URL is git's concern (ssh agent, credential helper). The
default `dir` is a directory named after the source (trailing `.git` stripped),
matching `git clone`.
