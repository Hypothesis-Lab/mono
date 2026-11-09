# Local services

Postgres, Temporal, SeaweedFS, and the Temporal UI run in `compose.yaml`. The
backend, generator, and frontend run as per-user systemd services so they can be
restarted without rebuilding or recreating the infrastructure containers. The
frontend serves the production build with Vite preview; run `npm run build`
after frontend changes before restarting it.

Install the tracked units:

```sh
mkdir -p ~/.config/systemd/user
ln -sfn /home/clanker/projects/startup/code/systemd/startup-generator.service ~/.config/systemd/user/startup-generator.service
ln -sfn /home/clanker/projects/startup/code/systemd/startup-backend.service ~/.config/systemd/user/startup-backend.service
ln -sfn /home/clanker/projects/startup/code/systemd/startup-frontend.service ~/.config/systemd/user/startup-frontend.service
systemctl --user daemon-reload
systemctl --user enable --now startup-generator.service startup-backend.service startup-frontend.service
```

Start the infrastructure separately with:

```sh
podman compose up -d
```

The native services listen on ports 5175 (frontend), 8000 (backend), and 8001
(generator). Temporal UI remains available on port 8080.

The units load credentials from the existing `.env` without copying them into
the unit files. Service logs are available with `journalctl --user`:

```sh
journalctl --user -u startup-backend.service -n 30 --no-pager
journalctl --user -u startup-generator.service -n 30 --no-pager
journalctl --user -u startup-frontend.service -n 30 --no-pager
```
