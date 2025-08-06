````markdown
## mango

[Foreman](https://github.com/ddollar/foreman) in Go, forked to add optional Loki-based log aggregation.

**Author:** jad21

---

### Installation

```bash
go install github.com/jad21/mango@latest
````

---

### Usage

```bash
$ cat Procfile
web:    bin/web start -p $PORT
worker: bin/worker queue=FOO

$ mango start
web    | listening on port 5000
worker | listening to queue FOO
```

Use `mango help` to list all commands, and `mango help <command>` for detailed help.

#### Loki Logging (opcional)

Configura tus logs hacia Grafana Loki en **`.mango`** o mediante flags:

1. **Archivo `.mango`:**

   ```ini
   procfile=Procfile
   port=5000
   concurrency=web=1,worker=2
   shutdown_grace_time=3

   # Loki (opcional)
   loki.url=http://localhost:3100/loki/api/v1/push
   loki.job=my-app
   ```

2. **Flags de CLI:**

   ```bash
   mango start \
     --loki.url=http://localhost:3100/loki/api/v1/push \
     --loki.job=my-app
   ```

Si `--loki.url` queda vacío, mango funcionará sin enviar logs a Loki.

---

### License

Apache 2.0 © 2015 David Dollar, © 2025 jad21
Fork incluye integración con Grafana Loki para agregación de logs.

