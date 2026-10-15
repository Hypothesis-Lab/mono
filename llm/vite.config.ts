import react from '@vitejs/plugin-react'
import { spawn } from 'node:child_process'
import { defineConfig, type ViteDevServer } from 'vite'

type OpenCodeRequest = {
  model?: string
  messages?: Array<{ role?: string; content?: string }>
}

function readBody(request: NodeJS.ReadableStream) {
  return new Promise<string>((resolve, reject) => {
    let body = ''
    request.setEncoding('utf8')
    request.on('data', (chunk: string) => {
      body += chunk
    })
    request.on('end', () => resolve(body))
    request.on('error', reject)
  })
}

function writeJson(response: NodeJS.WritableStream & { statusCode?: number; setHeader: (name: string, value: string) => void }, status: number, body: unknown) {
  response.statusCode = status
  response.setHeader('Content-Type', 'application/json')
  response.write(JSON.stringify(body))
  response.end()
}

function installOpenCodeProxy(server: ViteDevServer) {
  server.middlewares.use('/api/opencode', async (request, response, next) => {
    if (request.method !== 'POST') {
      next()
      return
    }

    try {
      const input = JSON.parse(await readBody(request)) as OpenCodeRequest
      const prompt = input.messages?.filter((message) => message.role === 'user').at(-1)?.content
      if (!prompt) {
        writeJson(response, 400, { error: { message: 'A user message is required' } })
        return
      }

      const model = input.model ?? 'cat-jam-free'
      const child = spawn('opencode', ['run', '--standalone', '--model', `opencode/${model}`, '--format', 'json', prompt], {
        cwd: process.cwd(),
        stdio: ['ignore', 'pipe', 'pipe'],
      })
      let stdout = ''
      let stderr = ''
      child.stdout.setEncoding('utf8')
      child.stderr.setEncoding('utf8')
      child.stdout.on('data', (chunk: string) => {
        stdout += chunk
      })
      child.stderr.on('data', (chunk: string) => {
        stderr += chunk
      })

      child.on('error', (error) => {
        writeJson(response, 503, { error: { message: `Could not start OpenCode: ${error.message}` } })
      })
      child.on('close', (code) => {
        const text = stdout
          .split('\n')
          .flatMap((line) => {
            try {
              const event = JSON.parse(line) as { type?: string; part?: { type?: string; text?: string } }
              return event.type === 'text' && event.part?.type === 'text' && event.part.text ? [event.part.text] : []
            } catch {
              return []
            }
          })
          .join('')
          .trim()

        if (code !== 0 || !text) {
          writeJson(response, 502, { error: { message: stderr.trim() || 'OpenCode returned no text' } })
          return
        }

        writeJson(response, 200, {
          choices: [{ message: { role: 'assistant', content: text } }],
          model,
        })
      })
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Invalid request'
      writeJson(response, 400, { error: { message } })
    }
  })
}

export default defineConfig({
  plugins: [react(), { name: 'opencode-proxy', configureServer: installOpenCodeProxy }],
})
