# LLM Lab

Small React/TypeScript page that runs a two-step greeting pipeline against
the local OpenCode client using `opencode/cat-jam-free`.

## Run locally

```sh
npm install
npm run dev
```

The Vite development server proxies `/api/opencode` to the installed
`opencode` CLI. OpenCode itself can use Cat Jam Free without a user API key.
The browser never receives an OpenCode credential.

The app expects `opencode` to be available on `PATH`:

```sh
opencode --version
```

For a production deployment, replace the Vite development middleware with a
server-side endpoint that invokes or otherwise securely authenticates
OpenCode. Do not put an OpenCode key in a `VITE_*` environment variable.
