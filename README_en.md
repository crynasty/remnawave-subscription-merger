[Русский](README.md) | English

# Remnawave Subscription Merger

Remnawave Subscription Merger is a reverse proxy that sits in front of a
[Remnawave](https://remna.st/) subscription page. It combines a user's primary and limited
subscription configurations into a single response while preserving the primary configuration's
metadata.

## Features

- Browser passthrough for the subscription page and its assets
- Support for all Remnawave client configuration formats
- Preservation of primary subscription metadata and configuration sections unrelated to proxies
- Fail-open behavior: merge failures fall back to the original primary configuration
- Compatibility with alternative subscription pages that return the same set of HTTP headers

## Requirements

| Dependency | Minimum version |
|---|---|
| Remnawave | 1.6.0 or later |

## Preparing Remnawave users

Each primary user must have a corresponding limited user. The limited username is formed using
`LIMITED_PREFIX`:

```text
Primary username: 0123456789
LIMITED_PREFIX: L
Limited username: L_0123456789
```

Keep `LIMITED_PREFIX` short: Remnawave limits the full username, including the prefix and `_`, to 36 characters.

Primary and limited users must belong to different internal squads with different available hosts.
If the internal squads contain duplicate hosts, those hosts will be duplicated in the merged
configuration.

## Environment variables

| Variable | Required | Description |
|---|---:|---|
| `REMNAWAVE_TOKEN` | Yes | Remnawave panel API token. Create one under **Panel → Settings → API Tokens** |
| `REMNAWAVE_PANEL_URL` | Yes | Base URL of the Remnawave panel API |
| `REMNAWAVE_HEADERS` | No | Additional panel request headers in `Key:Value;Key2:Value2` format |
| `LIMITED_PREFIX` | Yes | Prefix used to derive limited usernames. Specify it without an underscore; the merger inserts `_` automatically |
| `PROXY_PORT` | Yes | Port on which the merger listens. The supplied example uses `3030` |
| `SUBSCRIPTION_PAGE_INTERNAL_URL` | Yes | Internal URL of the subscription page to which incoming requests are proxied |
| `LOG_LEVEL` | No | Log detail level: `DEBUG`, `INFO`, `WARN`, or `ERROR`. The default is `INFO`; to log only errors, set it to `ERROR` |

See [`.env.example`](.env.example) for a configuration template.

## Installation guide

1. Create a new directory for the merger:

   ```bash
   mkdir /opt/remnawave-subscription-merger && cd /opt/remnawave-subscription-merger
   ```

2. Download the environment variable template:

   ```bash
   curl -o .env https://raw.githubusercontent.com/crynasty/remnawave-subscription-merger/main/.env.example
   ```

3. Configure the environment variables:

   ```bash
   nano .env
   ```

4. Download `docker-compose.yml`:

   ```bash
   curl -O https://raw.githubusercontent.com/crynasty/remnawave-subscription-merger/main/docker-compose.yml
   ```

5. Start the merger:

   ```bash
   docker compose pull && docker compose up -d && docker compose logs -f
   ```

6. Route the public subscription domain from remnawave-subscription-page to the merger.

   For example, when using Caddy:

    ```diff
   https://sub.domain.com {
      encode
   -  reverse_proxy * http://remnawave-subscription-page:3010
   +  reverse_proxy * remnawave-subscription-merger:3030
   }

   :443 {
      tls internal
      respond 204
   }
   ```

Apply the new configuration by restarting your reverse proxy.

## Alternative subscription pages

The merger can be used with any alternative Remnawave subscription page that returns a set of HTTP
headers identical to [subscription-page](https://github.com/remnawave/subscription-page).

## Practical use case

The merger can be used for flexible traffic limiting: since the limited user's quota is consumed
only when their hosts are used, you can set separate traffic limits for selected internal squads
without affecting the user's primary subscription.

Using the `{{TRAFFIC_USED}}` tag in the limited user's host names allows you to monitor traffic
consumption in the client application.
