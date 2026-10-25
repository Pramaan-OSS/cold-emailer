defmodule RealtimeWeb.UserSocketCredentialTest do
  use ExUnit.Case, async: true

  alias RealtimeWeb.UserSocket

  @ticket "eyJhbGciOiJIUzI1NiJ9.e30.sig"

  test "a ws ticket is read from the query string" do
    assert UserSocket.credential(%{"token" => @ticket}, %{}) == {:ok, @ticket}
  end

  test "an API key or OAuth token in the query string is still accepted, deprecated" do
    assert UserSocket.credential(%{"token" => "wmbly_abc"}, %{}) == {:ok, "wmbly_abc"}
    assert UserSocket.credential(%{"token" => "wmat_abc"}, %{}) == {:ok, "wmat_abc"}
  end

  test "the header wins over the query string" do
    info = %{x_headers: [{"x-coldemail-token", "wmbly_header"}]}
    assert UserSocket.credential(%{"token" => "wmbly_query"}, info) == {:ok, "wmbly_header"}
  end

  test "an API key is read from the x-coldemail-token header" do
    info = %{x_headers: [{"x-forwarded-for", "1.2.3.4"}, {"x-coldemail-token", "wmbly_abc"}]}
    assert UserSocket.credential(%{}, info) == {:ok, "wmbly_abc"}
  end

  test "no credential at all is missing" do
    assert UserSocket.credential(%{}, %{x_headers: []}) == {:error, :missing_token}
    assert UserSocket.credential(%{"token" => ""}, %{}) == {:error, :missing_token}
  end

  test "a refused handshake answers 403 with the reason it was refused for" do
    payload = %{code: 4003, reason: Realtime.Auth.error_message(:credential_in_query)}
    conn = UserSocket.handle_error(Plug.Test.conn(:get, "/socket/websocket"), payload)

    assert conn.status == 403
    assert Jason.decode!(conn.resp_body) == %{"code" => 4003, "reason" => payload.reason}
    assert conn.resp_body =~ "x-coldemail-token"
  end

  test "a used ticket and a signed-out session say what to do next" do
    assert Realtime.Auth.error_message(:ticket_used) =~ "mint a new one"
    assert Realtime.Auth.error_message(:session_revoked) =~ "sign in again"
  end

  test "socket params are filtered from logs" do
    filtered = Phoenix.Logger.filter_values(%{"token" => @ticket, "vsn" => "1.0.0"})
    assert filtered["token"] == "[FILTERED]"
    assert filtered["vsn"] == "1.0.0"
  end
end
