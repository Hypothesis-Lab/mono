import react from '@vitejs/plugin-react'
import { spawn } from 'node:child_process'
import { DatabaseSync } from 'node:sqlite'
import { mkdirSync } from 'node:fs'
import { readFile, readdir } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig, type ViteDevServer } from 'vite'
import { DEFAULT_PROMPTS, type PromptId } from './src/prompts.js'

const PRODUCT_DIR = resolve(dirname(fileURLToPath(import.meta.url)), '../agent-framework/product')
const DATA_DIR = resolve(dirname(fileURLToPath(import.meta.url)), 'data')
const PROMPTS_DB_PATH = resolve(DATA_DIR, 'prompts.sqlite')
const PROMPT_IDS = Object.keys(DEFAULT_PROMPTS) as PromptId[]

mkdirSync(DATA_DIR, { recursive: true })
const promptDatabase = new DatabaseSync(PROMPTS_DB_PATH)
promptDatabase.exec('PRAGMA busy_timeout = 5000')
promptDatabase.exec(`
  CREATE TABLE IF NOT EXISTS prompts (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    updated_at TEXT NOT NULL
  )
`)
const seedPrompt = promptDatabase.prepare('INSERT OR IGNORE INTO prompts (id, content, updated_at) VALUES (?, ?, ?)')
for (const id of PROMPT_IDS) seedPrompt.run(id, DEFAULT_PROMPTS[id], new Date().toISOString())

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

function readPrompts() {
  return promptDatabase.prepare('SELECT id, content, updated_at AS updatedAt FROM prompts ORDER BY id').all()
}

function installOpenCodeProxy(server: ViteDevServer) {
  server.middlewares.use('/api/prompts', async (request, response, next) => {
    if (request.method === 'GET') {
      writeJson(response, 200, { prompts: readPrompts() })
      return
    }
    if (request.method !== 'PUT') {
      next()
      return
    }

    try {
      const input = JSON.parse(await readBody(request)) as { id?: string; content?: string }
      if (!input.id || !PROMPT_IDS.includes(input.id as PromptId)) {
        writeJson(response, 400, { error: { message: 'Unknown prompt id' } })
        return
      }
      if (typeof input.content !== 'string' || !input.content.trim()) {
        writeJson(response, 400, { error: { message: 'Prompt content must be a non-empty string' } })
        return
      }
      promptDatabase
        .prepare('UPDATE prompts SET content = ?, updated_at = ? WHERE id = ?')
        .run(input.content, new Date().toISOString(), input.id)
      writeJson(response, 200, { prompt: readPrompts().find((prompt) => prompt.id === input.id) })
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Invalid prompt request'
      writeJson(response, 400, { error: { message } })
    }
  })

  server.middlewares.use('/api/product-context', async (request, response, next) => {
    if (request.method !== 'GET') {
      next()
      return
    }

    try {
      const files = (await readdir(PRODUCT_DIR))
        .filter((filename) => filename.endsWith('.md'))
        .sort()
      const contents = await Promise.all(files.map(async (filename) => ({
        name: filename,
        content: await readFile(resolve(PRODUCT_DIR, filename), 'utf8'),
      })))
      const context = contents
        .map(({ name, content }) => `\n===== ИСТОЧНИК: ${name} =====\n${content}\n`)
        .join('')
      writeJson(response, 200, { productName: 'Tempo', files: contents, context })
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Could not load product documents'
      writeJson(response, 500, { error: { message } })
    }
  })

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
