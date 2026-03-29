import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";
import { readdir, readFile } from "node:fs/promises";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { dirname } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROLES_DIR = join(__dirname, "roles");
const PROMPTS_DIR = join(__dirname, "prompts");
const LLAMA_BASE = process.env.LLAMA_BASE_URL || "http://localhost:8080";

function parseFrontmatter(content) {
  const match = content.match(/^---\n([\s\S]*?)\n---\n([\s\S]*)$/);
  if (!match) return { meta: {}, body: content };

  const meta = {};
  for (const line of match[1].split("\n")) {
    const idx = line.indexOf(":");
    if (idx === -1) continue;
    const key = line.slice(0, idx).trim();
    const val = line.slice(idx + 1).trim();
    meta[key] = val;
  }
  return { meta, body: match[2].trim() };
}

async function loadMdFile(dir, name) {
  const files = await readdir(dir);
  for (const file of files) {
    if (!file.endsWith(".md")) continue;
    const content = await readFile(join(dir, file), "utf-8");
    const { meta, body } = parseFrontmatter(content);
    const itemName = meta.name || file.replace(/\.md$/, "");
    if (itemName === name) {
      return {
        description: meta.description || name,
        model: meta.model || null,
        temperature: meta.temperature ? parseFloat(meta.temperature) : 0.7,
        body,
      };
    }
  }
  return null;
}

async function loadAllMdFiles(dir) {
  const items = new Map();
  let files;
  try {
    files = await readdir(dir);
  } catch {
    return items;
  }

  for (const file of files) {
    if (!file.endsWith(".md")) continue;
    const content = await readFile(join(dir, file), "utf-8");
    const { meta, body } = parseFrontmatter(content);
    const name = meta.name || file.replace(/\.md$/, "");
    items.set(name, {
      description: meta.description || name,
      model: meta.model || null,
      temperature: meta.temperature ? parseFloat(meta.temperature) : 0.7,
      body,
    });
  }
  return items;
}

const HTTP_TIMEOUT = 30_000;

async function chat(role, userMessage) {
  const body = {
    messages: [
      { role: "system", content: role.body },
      { role: "user", content: userMessage },
    ],
    temperature: role.temperature,
  };
  if (role.model) {
    body.model = role.model;
  }

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), HTTP_TIMEOUT);

  try {
    const res = await fetch(`${LLAMA_BASE}/v1/chat/completions`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      signal: controller.signal,
    });

    if (!res.ok) {
      const text = await res.text();
      throw new Error(`llama-server ${res.status}: ${text}`);
    }

    const data = await res.json();
    return data.choices[0].message.content;
  } finally {
    clearTimeout(timeout);
  }
}

async function main() {
  // Load once at startup to discover names for registration
  const roles = await loadAllMdFiles(ROLES_DIR);
  const prompts = await loadAllMdFiles(PROMPTS_DIR);
  const server = new McpServer({
    name: "local-llm",
    version: "1.0.0",
  });

  // Register tools — re-read the .md file on every call
  for (const [name, role] of roles) {
    server.tool(
      name,
      role.description,
      { message: z.string().describe("The prompt or code to send") },
      async ({ message }) => {
        const fresh = await loadMdFile(ROLES_DIR, name);
        const result = await chat(fresh || role, message);
        return { content: [{ type: "text", text: result }] };
      }
    );
  }

  // Register prompts — re-read the .md file on every request
  for (const [name, prompt] of prompts) {
    server.prompt(
      name,
      prompt.description,
      async () => {
        const fresh = await loadMdFile(PROMPTS_DIR, name);
        return {
          messages: [
            { role: "user", content: { type: "text", text: (fresh || prompt).body } },
          ],
        };
      }
    );
  }

  const transport = new StdioServerTransport();
  await server.connect(transport);
}

main();
