import Config

# General application configuration
config :realtime,
  port: String.to_integer(System.get_env("PORT") || "4000"),
  jwt_secret: System.get_env("JWT_SECRET") || "dev_secret_change_in_production",
  # Redis configuration
  redis_url: System.get_env("REDIS_URL") || "redis://localhost:6379/0",
  # Connection limits
  max_connections_per_user: String.to_integer(System.get_env("MAX_CONNECTIONS_PER_USER") || "10"),
  max_connections_per_ip: String.to_integer(System.get_env("MAX_CONNECTIONS_PER_IP") || "50"),
  max_connections_global: String.to_integer(System.get_env("MAX_CONNECTIONS_GLOBAL") || "100000"),
  # Rate limits (per minute)
  rate_limit_ws_message: String.to_integer(System.get_env("RATE_LIMIT_WS_MESSAGE") || "120"),
  rate_limit_ws_connect: String.to_integer(System.get_env("RATE_LIMIT_WS_CONNECT") || "30"),
  rate_limit_ws_join: String.to_integer(System.get_env("RATE_LIMIT_WS_JOIN") || "30"),
  rate_limit_ws_event: String.to_integer(System.get_env("RATE_LIMIT_WS_EVENT") || "60")

# Ecto Repo configuration (for API key validation)
config :realtime, Realtime.Repo,
  database: System.get_env("DATABASE_NAME") || "coldemail_dev",
  username: System.get_env("DATABASE_USER") || "postgres",
  password: System.get_env("DATABASE_PASSWORD") || "postgres",
  hostname: System.get_env("DATABASE_HOST") || "localhost",
  port: String.to_integer(System.get_env("DATABASE_PORT") || "5432"),
  pool_size: String.to_integer(System.get_env("DATABASE_POOL_SIZE") || "5")

config :realtime, ecto_repos: [Realtime.Repo]

# Phoenix Endpoint configuration
config :realtime, RealtimeWeb.Endpoint,
  url: [host: "localhost"],
  http: [
    port: String.to_integer(System.get_env("PORT") || "4000"),
    protocol_options: [invalid_response_headers: :error_terminate],
    transport_options: [socket_opts: [:inet6]]
  ],
  secret_key_base:
    System.get_env("SECRET_KEY_BASE") ||
      "dev_secret_key_base_min_64_chars_change_in_production_1234567890",
  pubsub_server: Realtime.PubSub

# Logger configuration
config :logger, :console,
  format: "$time $metadata[$level] $message\n",
  metadata: [:request_id, :user_id]

# Use Jason for JSON parsing
config :phoenix, :json_library, Jason

# Socket connect params are logged; any key containing these is shown as [FILTERED].
config :phoenix, :filter_parameters, ["password", "token", "secret", "key"]

# Import environment specific config
import_config "#{config_env()}.exs"
