import { useState, type FormEvent } from "react";
import { Link, Navigate, useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { Button } from "../components/Button";
import { Form } from "../components/Form";
import { Input } from "../components/Input";
import { apiMessage } from "../lib/api";

const strongPassword =
  /^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)(?=.*[^A-Za-z0-9]).{12,72}$/;

export function AuthPage({ mode }: { mode: "login" | "register" }) {
  const auth = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [validation, setValidation] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const register = mode === "register";

  if (auth.authenticated) return <Navigate to="/dashboard" replace />;

  async function submit(event: FormEvent) {
    event.preventDefault();
    const next: Record<string, string> = {};
    if (register && name.trim().length < 2)
      next.name = "Enter at least 2 characters.";
    if (!email.includes("@")) next.email = "Enter a valid email address.";
    if (!password) next.password = "Password is required.";
    if (register && !strongPassword.test(password)) {
      next.password =
        "Use 12–72 characters with upper/lowercase, a number, and a symbol.";
    }
    setValidation(next);
    if (Object.keys(next).length) return;
    setBusy(true);
    setError("");
    try {
      if (register) await auth.register(name, email, password);
      else await auth.login(email, password);
      const destination = (
        location.state as { from?: { pathname?: string } } | null
      )?.from?.pathname;
      navigate(destination ?? "/dashboard", { replace: true });
    } catch (requestError) {
      setError(apiMessage(requestError));
    } finally {
      setBusy(false);
    }
  }

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
        <Form className="auth-form" onSubmit={submit} error={error}>
          <p className="eyebrow">
            {register ? "START A WORKSPACE" : "WELCOME BACK"}
          </p>
          <h2>{register ? "Create your account" : "Sign in to TeamOps"}</h2>
          <p>
            {register
              ? "Bring your team and projects into focus."
              : "Pick up where your team left off."}
          </p>
          {register && (
            <Input
              label="Name"
              name="name"
              autoComplete="name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              error={validation.name}
            />
          )}
          <Input
            label="Email"
            name="email"
            type="email"
            autoComplete="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            error={validation.email}
          />
          <Input
            label="Password"
            name="password"
            type="password"
            autoComplete={register ? "new-password" : "current-password"}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            error={validation.password}
          />
          <Button busy={busy}>{register ? "Create account" : "Sign in"}</Button>
          <p className="switch">
            {register ? "Already have an account?" : "New to TeamOps?"}{" "}
            <Link to={register ? "/login" : "/register"}>
              {register ? "Sign in" : "Create an account"}
            </Link>
          </p>
        </Form>
      </section>
    </div>
  );
}
