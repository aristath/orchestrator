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

async function loadMdFiles(dir) {
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

  const res = await fetch(`${LLAMA_BASE}/v1/chat/completions`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });

  if (!res.ok) {
    const text = await res.text();
    throw new Error(`llama-server ${res.status}: ${text}`);
  }

  const data = await res.json();
  return data.choices[0].message.content;
}

async function main() {
  const roles = await loadMdFiles(ROLES_DIR);
  const prompts = await loadMdFiles(PROMPTS_DIR);
  const server = new McpServer({
    name: "local-llm",
    version: "1.0.0",
  });

  // Register tools from roles/*.md
  for (const [name, role] of roles) {
    server.tool(
      name,
      role.description,
      { message: z.string().describe("The prompt or code to send") },
      async ({ message }) => {
        const result = await chat(role, message);
        return { content: [{ type: "text", text: result }] };
      }
    );
  }

  // Register prompts from prompts/*.md
  for (const [name, prompt] of prompts) {
    server.prompt(
      name,
      prompt.description,
      async () => ({
        messages: [
          { role: "user", content: { type: "text", text: prompt.body } },
        ],
      })
    );
  }

  const transport = new StdioServerTransport();
  await server.connect(transport);
}

main();
