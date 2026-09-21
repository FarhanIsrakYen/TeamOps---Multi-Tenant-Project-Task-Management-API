// @vitest-environment jsdom
import "@testing-library/jest-dom/vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it } from "vitest";
import { AuthProvider } from "../auth/AuthContext";
import { session } from "../lib/api";
import { ProtectedRoute } from "./ProtectedRoute";

function renderRoutes() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/private"]}>
        <AuthProvider>
          <Routes>
            <Route path="/login" element={<p>Sign in</p>} />
            <Route element={<ProtectedRoute />}>
              <Route path="/private" element={<p>Private workspace</p>} />
            </Route>
          </Routes>
        </AuthProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

afterEach(() => session.clear());

describe("ProtectedRoute", () => {
  it("redirects an anonymous visitor", () => {
    renderRoutes();
    expect(screen.getByText("Sign in")).toBeInTheDocument();
  });

  it("renders protected content for an in-memory session", () => {
    session.set({
      accessToken: "access",
      refreshToken: "refresh",
      accessTokenExpiresAt: "2099-01-01T00:00:00Z",
      user: {
        id: "user-id",
        email: "person@example.com",
        name: "Person",
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
      },
    });
    renderRoutes();
    expect(screen.getByText("Private workspace")).toBeInTheDocument();
  });
});
