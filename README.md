# LoadShop

An in-memory electronics web shop built as a system-under-test for browser and protocol load testing.
It is one static Go binary that serves the JSON API (`/api/...`) and the React SPA (`/`) on one port.
There is no database and no Redis. The catalogue is shared and immutable, and carts and orders live in sharded in-memory maps with no TTLs, so a catalogue can never come back empty under load.

## Build and run

```sh
make web        # npm install + vite build -> web/dist (embedded into the binary)
make build      # native binary: bin/loadshop
make linux      # CGO_ENABLED=0 linux/amd64: bin/loadshop-linux-amd64
make test       # go vet + go test -race
make loadgen    # bin/loadgen (+ linux/amd64 build)
./bin/loadshop  # http://localhost:8000
```

Docker (distroless static, runs as nonroot): `make web linux && docker build --platform linux/amd64 -t loadshop .`, then `docker run -p 8000:8000 loadshop`.

## Flags

| Flag | Env | Default | Purpose |
|---|---|---|---|
| `-addr` | `LOADSHOP_ADDR` | `:8000` | listen address |
| `-secret` | `LOADSHOP_SECRET` | fixed dev secret | HMAC key for bearer tokens. Use the same value on every replica. |
| `-token-ttl` | `LOADSHOP_TOKEN_TTL` | `24h` | token lifetime |
| `-access-log` | `LOADSHOP_ACCESS_LOG=1` | off | per-request logging (off by default for speed) |

## API

Catalogue endpoints are public. Endpoints marked "auth" need `Authorization: Bearer <token>` and return 401 without it. Errors are always `{"error": "..."}`.

| Method and path | Notes |
|---|---|
| `POST /api/auth/login` `{username,password}` | Any non-empty values work. Returns `{access_token, token_type, username, expires_in}`. |
| `GET /api/auth/me` (auth) | `{username}` |
| `POST /api/auth/tokens` `{prefix,count}` | CSV `username,token` for `<prefix>1..<prefix>count` (count can be up to 100000) |
| `GET /api/products?limit=&offset=&category=&q=&sort=` | `{items,total,limit,offset}`. `limit` defaults to 20 (max 100). `sort` is one of `price_asc`, `price_desc`, `rating`, `name` (default: id). |
| `GET /api/products/{id}` | detail with specs. `{id}` can also be a slug. |
| `GET /api/products/{id}/image.svg` | generated image, cached as immutable |
| `GET /api/categories` | `[{slug,name,count}]` |
| `GET /api/cart` (auth) | `{items,itemCount,subtotal,tax,shippingCost,total}` |
| `POST /api/cart/items` `{productId,quantity}` (auth) | Adds to the existing line. `quantity` must be 1..100 and the line is capped at 999. |
| `PATCH /api/cart/items/{productId}` `{quantity}` (auth) | 0..999 (the line cap), where 0 removes the line |
| `DELETE /api/cart/items/{productId}` (auth) | idempotent |
| `POST /api/orders` (auth) | body `{shipping:{name,email,address,city,zip,country}, payment:{cardNumber}}`. Returns 201 with the order (`status: "confirmed"`) and empties the cart. |
| `GET /api/orders`, `GET /api/orders/{id}` (auth) | the caller's own orders, newest first |
| `POST /api/admin/reset` | clears all carts and orders (no auth). Tokens stay valid. |
| `GET /healthz` | `{"status":"ok"}` |
| `GET /metrics` | plain text: total, in-flight, uptime, and requests by route pattern and status class |

Tokens are stateless: `base64url("user|expiryUnix") + "." + base64url(HMAC-SHA256(secret, "user|expiryUnix"))`. Any replica with the same secret accepts them, and they survive restarts. Stock is shown for display only and is never decremented, so long runs never run out of stock.

Catalogue: 48 products in 6 categories, with ids 1..48 in a fixed order (embedded `internal/shop/seed.json`). Examples: id 1 is "Apple MacBook Air", id 9 is "Google Pixel 4", and id 22 is "Sonos Era 100".

## Token pool for a load test

```sh
curl -s -X POST localhost:8000/api/auth/tokens -d '{"prefix":"vu","count":500}' > tokens.csv
# username,token
# vu1,dnUxfDE3OT...
```

Browser virtual users can skip the login page. Set `localStorage.loadshop_token = <token>` (and optionally `loadshop_user = <username>`) before the app loads. If the username is missing, the app fetches it from `/api/auth/me`.

## UI routes and test ids

Routes: `/`, `/category/:slug`, `/product/:slug`, `/login?next=`, `/cart`, `/checkout`, `/orders`, `/orders/:id`. When you are logged out, cart, checkout and orders redirect to login.

| Area | `data-testid` |
|---|---|
| Header | `logo-link`, `search-input`, `search-submit`, `category-link-all`, `category-link-<cat>`, `cart-link`, `cart-count`, `login-link`, `user-menu`, `user-name`, `orders-link`, `logout-button` |
| Catalogue | `catalog-heading`, `catalog-count`, `sort-select`, `product-grid`, `product-card-<slug>`, `product-name-<slug>`, `product-brand-<slug>`, `product-price-<slug>`, `product-rating-<slug>`, `view-details-<slug>`, `add-to-cart-<slug>` |
| Detail | `back-to-catalog`, `product-detail`, `product-detail-name`, `product-detail-brand`, `product-detail-price`, `product-detail-image`, `product-detail-description`, `product-specs`, `quantity-decrease`, `quantity-input`, `quantity-increase`, `product-detail-add-to-cart`, `added-to-cart-notice`, `go-to-cart` |
| Cart | `cart-heading`, `cart-empty`, `cart-line-<slug>`, `cart-qty-<slug>`, `cart-remove-<slug>`, `cart-line-total-<slug>`, `cart-subtotal`, `cart-tax`, `cart-shipping`, `cart-total`, `checkout-button`, `continue-shopping` |
| Checkout | `checkout-name`, `checkout-email`, `checkout-address`, `checkout-city`, `checkout-zip`, `checkout-country`, `checkout-card`, `checkout-total`, `place-order` |
| Confirmation | `order-confirmation`, `order-id`, `order-status`, `order-total`, `view-orders` |
| Orders | `orders-heading`, `orders-table`, `order-row-<id>`, `order-link-<id>`, `orders-empty` |
| Login | `login-username`, `login-password`, `login-submit` |

Slugs are the lowercased names with dashes, for example `google-pixel-4` and `apple-macbook-air`.

## Load generator

```sh
./bin/loadgen -url http://localhost:8000 -c 100 -d 10s [-cart 10]
```

Each client gets its own minted token and alternates between `GET /api/products?limit=50` and `GET /api/products/{random}`. `-cart N` makes N% of the requests `POST /api/cart/items` instead. It prints req/s and p50, p90 and p99 latency.

## Layout

`main.go` (flags, embed, server), `internal/shop` (catalogue, auth, store, handlers, metrics, static, tests), `web/` (Vite + React + Tailwind), `tools/loadgen`.

## Running it in the Skyramp lab

LoadShop replaces the Demo Shop as the system under test on ghlab (`ubuntu@10.176.135.122`, 2 cores).
`bin/` ships prebuilt binaries, so a machine without Go or Node can still deploy it:
`bin/loadshop` (macOS arm64), `bin/loadshop-linux-amd64`, and the matching `bin/loadgen*`.

Deploy on ghlab. The image is built from the prebuilt binary and the Dockerfile:

```sh
ssh ubuntu@10.176.135.122 'mkdir -p ~/loadshop/bin'
scp bin/loadshop-linux-amd64 ubuntu@10.176.135.122:loadshop/bin/
scp Dockerfile ubuntu@10.176.135.122:loadshop/
ssh ubuntu@10.176.135.122 '
  cd ~/loadshop && docker build -q -t loadshop:latest .
  [ -f secret.env ] || echo "LOADSHOP_SECRET=$(openssl rand -hex 32)" > secret.env
  docker rm -f loadshop 2>/dev/null
  docker run -d --name loadshop --restart unless-stopped --env-file secret.env \
    -p 8000:8000 -p 5173:8000 loadshop:latest'
curl -s http://10.176.135.122:8000/healthz
```

The UI and API answer on `http://10.176.135.122:8000`. Port 5173 maps to the same server so the old Demo Shop UI address still works.
The Demo Shop containers use the same ports, so only one SUT runs at a time. Switch between them like this:

```sh
# to the Demo Shop
ssh ubuntu@10.176.135.122 'docker stop loadshop && docker start demoshop-redis demoshop-backend demoshop-frontend'
# to LoadShop
ssh ubuntu@10.176.135.122 'docker stop demoshop-backend demoshop-frontend demoshop-redis && docker start loadshop'
```

### Inputs for a fleet run

- **Credential pool.** The fleet's `CredentialPool.fromCsv` reads a `token` column. Mint the pool and keep only that column:
  `curl -s -X POST http://10.176.135.122:8000/api/auth/tokens -d '{"prefix":"lt","count":200}' | cut -d, -f2 > credentials.csv`
- **Storage state for recording.** Log in once with `POST /api/auth/login` using any credentials. Then write the token into a Playwright storage state for origin `http://10.176.135.122:8000`, under the localStorage keys `loadshop_token` and `loadshop_user`.
- **Tokens and restarts.** Tokens are signed with `LOADSHOP_SECRET`. They stay valid across restarts as long as `secret.env` is unchanged, and they expire after `-token-ttl` (24h by default).
- **Reset.** `POST /api/admin/reset` clears every cart and order between runs. The catalogue never changes.

### Measured on ghlab

- 18.6k req/s, p99 46 ms, 0 errors (`loadgen -c 200 -d 15s -cart 10` from win2).
- 10.4k req/s, 0 failures as the SUT of a 50-user fleet run from win1 and win3. In that run the workers saturated, not LoadShop.
