import { useState, type FormEvent } from "react";
import { Link, Navigate, useNavigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { apiMessage } from "../lib/api";

export function AuthPage({ mode }: { mode: "login" | "register" }) {
  const auth = useAuth();
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  if (auth.authenticated) return <Navigate to="/organizations" replace />;
  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      if (mode === "login") await auth.login(email, password);
      else await auth.register(name, email, password);
      navigate("/organizations");
    } catch (err) {
      setError(apiMessage(err));
    } finally {
      setBusy(false);
    }
  }
  const register = mode === "register";
  return (
    <div className="auth-page">
      <section className="auth-story">
        <Link className="brand" to="/">
          <span className="brand-mark">T</span>
          <span>TeamOps</span>
        </Link>
        <div>
          <p className="eyebrow">OPERATIONS, ALIGNED</p>
          <h1>
            Move work forward.
            <br />
            <em>Together.</em>
          </h1>
          <p>
            One focused workspace for projects, decisions, and the work that
            matters.
          </p>
        </div>
        <footer>Built for teams that value clarity.</footer>
      </section>
      <section className="auth-form-wrap">
        <form className="auth-form" onSubmit={submit}>
          <p className="eyebrow">
            {register ? "START A WORKSPACE" : "WELCOME BACK"}
          </p>
          <h2>{register ? "Create your account" : "Sign in to TeamOps"}</h2>
          <p>
            {register
              ? "Bring your team and projects into focus."
              : "Pick up where your team left off."}
          </p>
          {error && (
            <div className="error-banner" role="alert">
              {error}
            </div>
          )}
          {register && (
            <label>
              Name
              <input
                autoComplete="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                minLength={2}
              />
            </label>
          )}
          <label>
            Email
            <input
              type="email"
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
            />
          </label>
          <label>
            Password
            <input
              type="password"
              autoComplete={register ? "new-password" : "current-password"}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={register ? 10 : 1}
            />
          </label>
          <button className="primary" disabled={busy}>
            {busy ? "Please wait…" : register ? "Create account" : "Sign in"}
          </button>
          <p className="switch">
            {register ? "Already have an account?" : "New to TeamOps?"}{" "}
            <Link to={register ? "/login" : "/register"}>
              {register ? "Sign in" : "Create an account"}
            </Link>
          </p>
        </form>
      </section>
    </div>
  );
}
