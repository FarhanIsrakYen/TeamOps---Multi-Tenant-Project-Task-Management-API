import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
export function Layout() {
  const auth = useAuth();
  const navigate = useNavigate();
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <NavLink className="brand" to="/dashboard">
          <span className="brand-mark">T</span>
          <span>TeamOps</span>
        </NavLink>
        <nav>
          <NavLink to="/dashboard">Dashboard</NavLink>
          <NavLink to="/organizations">Organizations</NavLink>
          <NavLink to="/profile">Profile</NavLink>
        </nav>
        <div className="account">
          <span className="avatar">
            {auth.user?.name.slice(0, 1).toUpperCase()}
          </span>
          <div>
            <strong>{auth.user?.name}</strong>
            <small>{auth.user?.email}</small>
          </div>
          <button
            className="link-button"
            onClick={async () => {
              await auth.logout();
              navigate("/login");
            }}
          >
            Sign out
          </button>
        </div>
      </aside>
      <main className="main">
        <Outlet />
      </main>
    </div>
  );
}
