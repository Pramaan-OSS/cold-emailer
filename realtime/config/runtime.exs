import Config

if config_env() == :prod do
  # Required environment variables
  jwt_secret =
    System.get_env("JWT_SECRET") ||
      raise "JWT_SECRET environment variable is required"

  # Same floor the backend applies to AUTH_SECRET, which is this same value.
  # HS256 keys shorter than the hash output can be recovered offline from any
  # token the service has issued.
  if byte_size(jwt_secret) < 32 do
    raise "JWT_SECRET must be at least 32 bytes: it verifies every session token. Generate one with: make gen-key"
  end

  secret_key_base =
    System.get_env("SECRET_KEY_BASE") ||
      raise "SECRET_KEY_BASE environment variable is required"

  database_url =
    System.get_env("DATABASE_URL") ||
      raise "DATABASE_URL environment variable is required"

  port = String.to_integer(System.get_env("PORT") || "4000")
  host = System.get_env("PHX_HOST") || "localhost"

  config :realtime,
    port: port,
    jwt_secret: jwt_secret,
    # Redis configuration
    redis_url: System.get_env("REDIS_URL") || "redis://localhost:6379/0",
    # Connection limits
    max_connections_per_user:
      String.to_integer(System.get_env("MAX_CONNECTIONS_PER_USER") || "10"),
    max_connections_per_ip: String.to_integer(System.get_env("MAX_CONNECTIONS_PER_IP") || "50"),
    max_connections_global:
      String.to_integer(System.get_env("MAX_CONNECTIONS_GLOBAL") || "100000"),
    # Rate limits (per minute)
    rate_limit_ws_message: String.to_integer(System.get_env("RATE_LIMIT_WS_MESSAGE") || "120"),
    rate_limit_ws_connect: String.to_integer(System.get_env("RATE_LIMIT_WS_CONNECT") || "30"),
    rate_limit_ws_join: String.to_integer(System.get_env("RATE_LIMIT_WS_JOIN") || "30"),
    rate_limit_ws_event: String.to_integer(System.get_env("RATE_LIMIT_WS_EVENT") || "60")

  check_origin =
    case System.get_env("CHECK_ORIGIN_HOSTS", "") |> String.split(",", trim: true) do
      [] -> System.get_env("CHECK_ORIGIN", "false") == "true"
      origins -> Enum.map(origins, &String.trim/1)
    end

  config :realtime, RealtimeWeb.Endpoint,
    url: [host: host, port: 443, scheme: "https"],
    http: [
      ip: {0, 0, 0, 0, 0, 0, 0, 0},
      port: port,
      protocol_options: [invalid_response_headers: :error_terminate]
    ],
    secret_key_base: secret_key_base,
    # The browser's Origin on a websocket upgrade is the DASHBOARD's origin,
    # not this service's. check_origin: true compares against url: [host: ...],
    # which is PHX_HOST, which is the websocket host, so it refuses every real
    # connection. The transport used to hardcode check_origin: false, which hid
    # that; now that the setting actually applies, it has to be given the right
    # answer rather than a boolean.
    #
    # CHECK_ORIGIN_HOSTS is a comma-separated list of allowed origins, e.g.
    # "https://app.example.com". Set it and the check is real. CHECK_ORIGIN=true
    # without it keeps the old host-based behaviour for anyone relying on it.
    check_origin: check_origin

  # Postgrex verifies the server against the system CA store, which has no
  # Amazon RDS root in it, so an RDS database needs DATABASE_SSL_CA_FILE
  # pointing at a bundle. The image ships AWS's at
  # /etc/ssl/rds/global-bundle.pem. It is opt-in rather than the default for
  # the same reason the backend makes sslrootcert opt-in: pointing every
  # install at an RDS-only store would break a Postgres fronted by a public CA.
  #
  # A name that is present but blank counts as unset, the same rule the error
  # DSN below follows: compose passes every optional variable through as "", and
  # `cacertfile: ~c""` with verify_peer is no CA source at all, so it fails the
  # handshake instead of falling back to the system store.
  database_ssl_ca_file =
    case System.get_env("DATABASE_SSL_CA_FILE") do
      value when is_binary(value) ->
        case String.trim(value) do
          "" -> nil
          trimmed -> trimmed
        end

      _ ->
        nil
    end

  database_ssl =
    cond do
      System.get_env("DATABASE_SSL", "true") != "true" ->
        false

      database_ssl_ca_file ->
        [
          verify: :verify_peer,
          cacertfile: to_charlist(database_ssl_ca_file),
          customize_hostname_check: [
            match_fun: :public_key.pkix_verify_hostname_match_fun(:https)
          ]
        ]

      true ->
        true
    end

  # Ecto Repo configuration (for API key validation)
  config :realtime, Realtime.Repo,
    url: database_url,
    ssl: database_ssl,
    pool_size: String.to_integer(System.get_env("DATABASE_POOL_SIZE") || "10"),
    # Left off deliberately: this prints the whole Repo config, password
    # included, into the logs the first time Postgres is unreachable.
    show_sensitive_data_on_connection_error: false

  # Error reporting. An env var that is present but empty must behave as unset:
  # compose passes every optional variable through as "" so a single .env can
  # to take the whole node down at boot.
  # A variable that is present but blank counts as unset here too, for the same
  # reason the DSN does: compose passes every optional variable through as "",
  # and System.get_env/2 only applies its default when the name is absent.
  env_or = fn name, fallback ->
    case System.get_env(name) do
      value when is_binary(value) ->
        case String.trim(value) do
          "" -> fallback
          trimmed -> trimmed
        end

      _ ->
        fallback
    end
  end

end
