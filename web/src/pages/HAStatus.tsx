import { useEffect, useState } from "react";
import { api, HAStatus as HAStatusType } from "../api";

// HAStatus is the admin-only High Availability view: storage/HA topology, this
// pod's role, the current leader, and the s3 fencing token. Polls every 5s so a
// failover (leader change) is visible without a manual refresh.
export function HAStatus() {
  const [st, setSt] = useState<HAStatusType | null>(null);
  const [error, setError] = useState("");

  const load = () =>
    api.getHA().then((s) => { setSt(s); setError(""); }).catch((e) => setError((e as Error).message));

  useEffect(() => {
    load();
    const t = setInterval(load, 5000);
    return () => clearInterval(t);
  }, []);

  const mono = { fontFamily: "ui-monospace, monospace" } as const;

  return (
    <div className="panel">
      <div className="inline" style={{ justifyContent: "space-between" }}>
        <h2 style={{ marginBottom: 0 }}>High Availability</h2>
        <span className="muted">auto-refresh · 5s</span>
      </div>
      {error && <div className="error">{error}</div>}
      {!st ? (
        <p className="muted">Loading…</p>
      ) : (
        <table style={{ marginTop: 12 }}>
          <tbody>
            <tr><th style={{ width: 180 }}>Mode</th><td>{st.mode}</td></tr>
            <tr><th>Storage backend</th><td>{st.backend}</td></tr>
            <tr><th>Leader election</th><td>{st.enabled ? "enabled" : "disabled (single instance)"}</td></tr>
            <tr><th>This pod</th><td style={mono}>{st.identity || "—"}</td></tr>
            <tr>
              <th>Role</th>
              <td>
                <span
                  className="badge"
                  style={st.is_leader
                    ? { color: "#22c55e", borderColor: "#22c55e" }
                    : undefined}
                >
                  {st.role || "—"}
                </span>
                {st.is_leader && <span className="muted" style={{ marginLeft: 6 }}>serving traffic</span>}
              </td>
            </tr>
            <tr><th>Current leader</th><td style={mono}>{st.leader || "—"}</td></tr>
            {st.lease_name && <tr><th>Lease</th><td style={mono}>{st.lease_name}</td></tr>}
            {typeof st.fencing_token === "number" && st.fencing_token > 0 && (
              <tr><th>Fencing token</th><td>{st.fencing_token}</td></tr>
            )}
          </tbody>
        </table>
      )}
      <p className="muted" style={{ marginTop: 12 }}>
        In HA only the leader serves traffic (the Service routes to the leader pod); standby pods stay
        Ready and take over automatically on failover. The fencing token guards object-storage metadata
        against a superseded leader.
      </p>
    </div>
  );
}
