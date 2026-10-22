defmodule Realtime.ErrorReporter do
 require Logger
 def capture_exception(exception, _opts \\ []), do: Logger.error("[issue-local][realtime] #{Exception.message(exception)}")
 def capture_message(message, _opts \\ []), do: Logger.error("[issue-local][realtime] #{message}")
end
