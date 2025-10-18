# Why, how, and why now?

Two weeks ago, I was at the Neon annual offsite when [PlanetScale announced support for the Neon serverless driver](https://planetscale.com/changelog/neon-serverless-driver-support) in their Postgres offering. That made me think: why shouldn't other Postgres setups like self-managed instances on a VPS, a cluster on Kubernetes, or even a local Postgres container also work with the Neon serverless driver?

---

So, what exactly is the Neon serverless driver?
I'll keep this short and link to a few good posts from Neon for deeper reading.

Some popular serverless environments restrict the use of raw TCP sockets, which regular Postgres drivers need to communicate using the Postgres wire protocol. While [Cloudflare Workers now support outbound TCP](https://developers.cloudflare.com/workers/runtime-apis/tcp-sockets/) connections and even provide an [impressive tech](https://developers.cloudflare.com/hyperdrive/) under the hood [1], Vercel Edge Functions [still do not](https://vercel.com/docs/functions/limitations). This limitation makes it impossible for these environments to use traditional Postgres drivers that rely on TCP.

The [Neon serverless driver](https://github.com/neondatabase/serverless), and more specifically [its proxy](https://github.com/neondatabase/neon/tree/main/proxy/src), solves this problem. The Neon multi-tenant proxy speaks both the Postgres wire protocol and SQL-over-HTTP or WebSockets. The JavaScript driver can then use HTTP or WebSockets as its transport layer, avoiding direct TCP connections altogether.

This architecture also enables the proxy to handle connection pooling, reducing the cost of repeatedly opening new Postgres connections and improving performance in serverless environments.

SQL-over-HTTP is straightforward: SQL queries are sent in the JSON payload of a POST request, the proxy interprets the query, and executes it on the target database. There is a small overhead for JSON serialization, but nothing significant.
WebSockets, on the other hand, establish a bidirectional channel where Postgres wire protocol packets are sent directly, without additional marshalling. Once the connection is open, packets are simply passed through.

Some good reads on the serverless driver and proxy:
- [Neon Docs: Serverless Driver](https://neon.com/docs/serverless/serverless-driver)
- [Neon Blog: Serverless Driver GA](https://neon.com/blog/serverless-driver-ga)
- [HTTP vs WebSockets for Postgres at the Edge](https://neon.com/blog/http-vs-websockets-for-postgres-queries-at-the-edge)

---

This is a clever idea, especially for environments with limited networking capabilities. That is when I started thinking about building a lightweight, standalone proxy that is provider-agnostic but works in the same way. The idea is to package it both as a binary and a Go library, making it easy to run standalone or embed into an existing service.

The initial goal is to achieve full compatibility with the JavaScript serverless driver and replicate its semantics. Once that is done, I have a few ideas for the standalone version:

- Connection string whitelisting: allow defining which connection strings are permitted, either by regex or full hostname.
- A ctl utility to manage proxy configuration at runtime.
- A Helm chart for quick Kubernetes deployment.
- Configurable proxy parameters such as max-payload, exposed endpoint, and which transport modes to enable (HTTP, WebSocket, or both).

---

Notes:

[1] Cloudflare's Hyperdrive acts as a globally distributed connection pooler and cache, improving performance by pooling connections and caching frequent queries instead of opening a TCP connection on every function run.