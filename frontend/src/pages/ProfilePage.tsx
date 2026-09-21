import { useAuth } from "../auth/AuthContext";
import { ErrorState } from "../components/ErrorState";
import { Loading } from "../components/Loading";
import { useMe } from "../hooks/queries";
import { formatDate, initials } from "../lib/format";

export function ProfilePage() {
  const auth = useAuth();
  const me = useMe();
  if (me.isLoading) return <Loading label="Loading profile" />;
  if (me.error || !me.data)
    return <ErrorState error={me.error} retry={() => void me.refetch()} />;
  return (
    <>
      <header className="page-header">
        <div>
          <p className="eyebrow">ACCOUNT</p>
          <h1>Profile</h1>
          <p>Your authenticated TeamOps identity.</p>
        </div>
      </header>
      <section className="profile-card">
        <div className="profile-avatar">{initials(me.data.name)}</div>
        <div>
          <h2>{me.data.name}</h2>
          <p>{me.data.email}</p>
        </div>
        <dl className="details">
          <div>
            <dt>User ID</dt>
            <dd>
              <code>{me.data.id}</code>
            </dd>
          </div>
          <div>
            <dt>Joined</dt>
            <dd>{formatDate(me.data.createdAt)}</dd>
          </div>
          <div>
            <dt>Updated</dt>
            <dd>{formatDate(me.data.updatedAt)}</dd>
          </div>
        </dl>
      </section>
      <section className="security-note">
        <h2>Session security</h2>
        <p>
          Access and refresh tokens are held only in memory and are never
          written to localStorage. Closing or reloading this tab ends the
          browser session. The backend remains the authority for every
          permission check.
        </p>
        <button
          className="button secondary compact"
          onClick={() => void auth.logout()}
        >
          Sign out all session data
        </button>
      </section>
    </>
  );
}
