defmodule Realtime.LocalSessionTest do
  use ExUnit.Case, async: true
  alias Realtime.Auth

  defp sign(claims, secret \\ "test_jwt_secret") do
    JOSE.JWT.sign(JOSE.JWK.from_oct(secret), %{"alg" => "HS256"}, claims)
    |> JOSE.JWS.compact()
    |> elem(1)
  end

  test "the session binding must come from a valid socket ticket" do
    sid = Ecto.UUID.generate()

    claims = %{
      "sub" => Ecto.UUID.generate(),
      "purpose" => "ws",
      "exp" => System.system_time(:second) + 600,
      "local_session_id" => sid
    }

    assert Auth.local_session(sign(claims)) == sid
    assert Auth.local_session(sign(claims, "wrong-secret")) == nil
    assert Auth.local_session(sign(Map.put(claims, "purpose", "access"))) == nil
    assert Auth.local_session(sign(Map.put(claims, "exp", 1))) == nil
    assert Auth.local_session(sign(Map.delete(claims, "local_session_id"))) == nil
    assert Auth.local_session(sign(Map.put(claims, "local_session_id", "invalid"))) == nil
  end
end
