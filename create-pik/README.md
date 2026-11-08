# create-pik

Create a project from an [agent-ready starter template](https://github.com/opikdev/starters):

```sh
npm create pik@latest
npm create pik@latest -- api-hono-starter my-api --yes
```

It downloads the matching [`pik`](https://github.com/opikdev/pik) release, verifies its SHA-256 checksum, caches it, and runs `pik new` with your arguments. You need Node 20 or later and `tar`.
