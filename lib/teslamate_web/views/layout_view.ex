defmodule TeslaMateWeb.LayoutView do
  use TeslaMateWeb, :view

  import Phoenix.Component
  use PhoenixHTMLHelpers

  dashboards =
    for dashboard_path <- Path.wildcard("grafana/dashboards/*.json") do
      @external_resource Path.relative_to_cwd(dashboard_path)

      dashboard_path
      |> File.read!()
      |> Jason.decode!()
      |> Map.take(["title", "uid"])
    end

  @dashboards Enum.sort_by(dashboards, & &1["title"])
  defp list_dashboards, do: @dashboards

  # 在这里定义 get_admin_dashboard_url
  def get_admin_dashboard_url(conn) do
    "#{conn.scheme}://#{conn.host}:5050"
  end
end
